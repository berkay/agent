// Package cmd is responsible for bootstrapping the agent, starting GO routines
// for different tasks in Neptune.io agent. The main function should directly call
// MainLoop() to start an agent instance.
package cmd

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/neptuneio/agent"
	"github.com/neptuneio/agent/logging"

	"path/filepath"
)

const (
	// Defaults for registration interval. We might make these configurable in future, if need be.
	heartbeatInterval      = time.Second * 5 * 60  // Heartbeat once every five minutes
	logsUploadInterval     = time.Second * 2 * 60  // Upload logs once every two minutes if the log changes.
	reregistrationInterval = time.Second * 60 * 60 // Re-register once every hour
)

var (
	endPoint         string
	apiKey           string
	configFilePath   string
	registrationInfo *agent.RegistrationInfo
)

func init() {
	flag.StringVar(&endPoint, "endpoint", "", "Neptune.io's API endpoint at which the agent should register.")
	flag.StringVar(&apiKey, "api_key", "", "Neptune.io api key for your account. Get this from Neptune.io app.")
	flag.StringVar(&configFilePath, "config", "", "Path to the agent config file.")
}

// Function to validate the NeptuneConfig object.
func validateConfig(configObj agent.NeptuneConfig) error {
	if len(configObj.ApiKey) == 0 {
		return errors.New("Neptune.io API key is missing.")
	}

	if len(configObj.Endpoint) == 0 {
		return errors.New("Neptune.io endpoint is missing.")
	}

	return nil
}

// Function to register the agent with Neptune.io service for first time when agent comes up.
func registerAgent(metaData *agent.HostMetaData, neptuneConfig *agent.NeptuneConfig, regInfoUpdatesCh chan<- string) {
	response, e := agent.RegisterAgent(*metaData, neptuneConfig)
	if e != nil {
		logging.Error("Could not register the agent.", logging.Fields{"error": e})
		agent.UpdateStatus(agent.RegistrationFailed)
		return
	}

	// Copy the new registration info to the global object so that all go routines
	// start using that info.
	if len(response.AgentId) > 0 {
		*registrationInfo = *response
		regInfoUpdatesCh <- "updated"
	} else {
		agent.UpdateStatus(agent.RegistrationFailed)
		logging.Error("Received incomplete registration response.", logging.Fields{"response": *response})
	}
}

// Main function for the agent which does the bootstrapping and starting all workers.
func MainLoop(errorChannel chan error, exitChannel chan struct{}) error {
	// Parse the commandline flags.
	flag.Parse()

	if len(configFilePath) == 0 {
		// Get the full path of the binary and pick the config file from the same directory.
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err == nil {
			configFilePath = filepath.Join(dir, agent.DefaultConfigFileName)
		} else {
			errorChannel <- err
			agent.ReportError(fmt.Sprintf("Could not get the absolute path of the installer. Error: %v", err))
			agent.UpdateStatus(agent.ConfigReadFailed)
		}
	}

	// Construct a config object from the flags passed to the agent.
	cmdlineConfig := agent.NeptuneConfig{Endpoint: endPoint, ApiKey: apiKey}
	neptuneConfig, agentConfig, err := agent.GetConfig(configFilePath, cmdlineConfig, errorChannel)
	if err != nil {
		fmt.Println("Invalid config file.", err)
		agent.UpdateStatus(agent.ConfigReadFailed)
		os.Exit(1)
	}

	if e := validateConfig(neptuneConfig); e != nil {
		errorChannel <- e
		agent.ReportError(fmt.Sprintf("Invalid config values. Error: %v", e))
		fmt.Printf("Invalid config values. Error: %v\n", e)
		agent.UpdateStatus(agent.ConfigReadFailed)
		os.Exit(1)
	} else {
		agent.UpdateStatus(agent.ConfigReadSucceeded)
	}

	// Get the absolute path of the log file and use that to setup logging.
	logFilePath := agentConfig.LogFile
	if !filepath.IsAbs(agentConfig.LogFile) {
		dir := filepath.Dir(configFilePath)
		logFilePath = filepath.Join(dir, logFilePath)
	}

	err = logging.SetupLogger(logFilePath, agentConfig.DebugMode, agent.ErrorsChannel)
	if err != nil {
		errorChannel <- err
		agent.ReportError(fmt.Sprintf("Could not setup logger. Error: %v", err))
	}

	logging.Info("Starting Neptune agent....", logging.Fields{"version": agent.AgentVersion})
	logging.Debug("Final config.", logging.Fields{"config": neptuneConfig})

	// Get the host metadata to register the agent.
	metaData, e := agent.GetHostMetaData(&agentConfig)
	if e != nil {
		logging.Error("Could not get metadata from host.", logging.Fields{"error": e})
		os.Exit(1)
	}

	i := 0
	for {
		registrationInfo, e = agent.RegisterAgent(metaData, &neptuneConfig)
		i += 1
		if e != nil {
			sleepDelay := math.Min(float64(i*30), 300)
			logging.Error("Could not register the agent. Retrying..", logging.Fields{"error": e, "delay": sleepDelay})
			time.Sleep(time.Second * time.Duration(sleepDelay))
		} else {
			break
		}
	}

	// Check if the registration has succeeded.
	if len(registrationInfo.AgentId) > 0 {
		agent.UpdateStatus(agent.RegistrationSucceeded)
	}

	// Set the registration info the apis.
	agent.SetRegistrationInfo(registrationInfo, metaData, neptuneConfig)

	// Initialize the events file cleaner.
	agent.InitializeEventsFile(filepath.Dir(configFilePath))

	heartbeatTickerCh := time.NewTicker(heartbeatInterval).C
	uploadLogsTickerCh := time.NewTicker(logsUploadInterval).C
	registrationTickerCh := time.NewTicker(reregistrationInterval).C

	// Upload the logs once in the beginning.
	e = agent.UploadLogs(&neptuneConfig, logFilePath, registrationInfo.AgentId)
	if e != nil {
		logging.Warn("Could not upload logs.", logging.Fields{"error": e})
	}

	regInfoUpdatesCh := make(chan string, 5)
	triggerReregistrationCh := make(chan time.Time, 5)

	// Start a GO routine to handle periodic agent registration, heartbeats and log uploads.
	go func() {
		for {
			select {
			case <-heartbeatTickerCh:
				e := agent.Beat(&neptuneConfig, registrationInfo.AgentId)
				if e != nil {
					logging.Error("Could not send heartbeats.", logging.Fields{"error": e})
				}

			case <-uploadLogsTickerCh:
				e := agent.UploadLogs(&neptuneConfig, logFilePath, registrationInfo.AgentId)
				if e != nil {
					logging.Warn("Could not upload logs.", logging.Fields{"error": e})
				}

			case <-triggerReregistrationCh:
				logging.Info("Retriggering the registration.", nil)
				registerAgent(&metaData, &neptuneConfig, regInfoUpdatesCh)

			case <-registrationTickerCh:
				registerAgent(&metaData, &neptuneConfig, regInfoUpdatesCh)
			}
		}
	}()

	events := make(chan *agent.Event, 10)
	actionOutputs := make(chan *agent.ActionOutputMessage, 10)

	// Start a GO routine to process SQS messages in an infinite loop.
	go func() {
		agent.RunLoop(registrationInfo, regInfoUpdatesCh, events, triggerReregistrationCh)
	}()

	// Start a GO routine to execute the runbooks handed over by the SQS worker.
	go func() {
		for event := range events {
			// Execute each action in a separate go routine.
			go func() {
				agent.ExecuteAction(event, registrationInfo, actionOutputs, agentConfig.GithubApiKey)
			}()
		}
	}()

	// Start a GO routine to send the runbook execution results to Neptune.io service.
	go func() {
		for actionOutput := range actionOutputs {
			if e := agent.SendActionOutput(&neptuneConfig, actionOutput); e != nil {
				logging.Error("Could not send action output to Neptune.", logging.Fields{"error": e})
			}
		}
	}()

	<-exitChannel

	return nil
}
