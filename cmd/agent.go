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

	"github.com/neptuneio/agent/api"
	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/executor"
	"github.com/neptuneio/agent/logging"
	"github.com/neptuneio/agent/metadata"
	"github.com/neptuneio/agent/state"
	"github.com/neptuneio/agent/worker"

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
	registrationInfo *api.RegistrationInfo
)

func init() {
	flag.StringVar(&endPoint, "endpoint", "", "Neptune.io's API endpoint at which the agent should register.")
	flag.StringVar(&apiKey, "api_key", "", "Neptune.io api key for your account. Get this from Neptune.io app.")
	flag.StringVar(&configFilePath, "config", "", "Path to the agent config file.")
}

// Function to validate the NeptuneConfig object.
func validateConfig(configObj config.NeptuneConfig) error {
	if len(configObj.ApiKey) == 0 {
		return errors.New("Neptune.io API key is missing.")
	}

	if len(configObj.Endpoint) == 0 {
		return errors.New("Neptune.io endpoint is missing.")
	}

	return nil
}

// Function to register the agent with Neptune.io service for first time when agent comes up.
func registerAgent(metaData *metadata.HostMetaData, neptuneConfig *config.NeptuneConfig, regInfoUpdatesCh chan<- string) {
	response, e := api.RegisterAgent(*metaData, neptuneConfig)
	if e != nil {
		logging.Error("Could not register the agent.", logging.Fields{"error": e})
		api.UpdateStatus(api.RegistrationFailed)
		return
	}

	// Copy the new registration info to the global object so that all go routines
	// start using that info.
	if len(response.AgentId) > 0 {
		*registrationInfo = *response
		regInfoUpdatesCh <- "updated"
	} else {
		api.UpdateStatus(api.RegistrationFailed)
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
			configFilePath = filepath.Join(dir, config.DefaultConfigFileName)
		} else {
			errorChannel <- err
			api.ReportError(fmt.Sprintf("Could not get the absolute path of the installer. Error: %v", err))
			api.UpdateStatus(api.ConfigReadFailed)
		}
	}

	// Construct a config object from the flags passed to the agent.
	cmdlineConfig := config.NeptuneConfig{Endpoint: endPoint, ApiKey: apiKey}
	neptuneConfig, agentConfig, err := config.GetConfig(configFilePath, cmdlineConfig, errorChannel)
	if err != nil {
		fmt.Println("Invalid config file.", err)
		api.UpdateStatus(api.ConfigReadFailed)
		os.Exit(1)
	}

	if e := validateConfig(neptuneConfig); e != nil {
		errorChannel <- e
		api.ReportError(fmt.Sprintf("Invalid config values. Error: %v", e))
		fmt.Printf("Invalid config values. Error: %v\n", e)
		api.UpdateStatus(api.ConfigReadFailed)
		os.Exit(1)
	} else {
		api.UpdateStatus(api.ConfigReadSucceeded)
	}

	// Get the absolute path of the log file and use that to setup logging.
	logFilePath := agentConfig.LogFile
	if !filepath.IsAbs(agentConfig.LogFile) {
		dir := filepath.Dir(configFilePath)
		logFilePath = filepath.Join(dir, logFilePath)
	}

	err = logging.SetupLogger(logFilePath, agentConfig.DebugMode, api.ErrorsChannel)
	if err != nil {
		errorChannel <- err
		api.ReportError(fmt.Sprintf("Could not setup logger. Error: %v", err))
	}

	logging.Info("Starting Neptune agent....", logging.Fields{"version": api.AgentVersion})
	logging.Debug("Final config.", logging.Fields{"config": neptuneConfig})

	// Get the host metadata to register the agent.
	metaData, e := metadata.GetHostMetaData(&agentConfig)
	if e != nil {
		logging.Error("Could not get metadata from host.", logging.Fields{"error": e})
		os.Exit(1)
	}

	i := 0
	for {
		registrationInfo, e = api.RegisterAgent(metaData, &neptuneConfig)
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
		api.UpdateStatus(api.RegistrationSucceeded)
	}

	// Set the registration info the apis.
	api.SetRegistrationInfo(registrationInfo, metaData, neptuneConfig)

	// Initialize the events file cleaner.
	state.InitializeEventsFile(filepath.Dir(configFilePath))

	heartbeatTickerCh := time.NewTicker(heartbeatInterval).C
	uploadLogsTickerCh := time.NewTicker(logsUploadInterval).C
	registrationTickerCh := time.NewTicker(reregistrationInterval).C

	// Upload the logs once in the beginning.
	e = api.UploadLogs(&neptuneConfig, logFilePath, registrationInfo.AgentId)
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
				e := api.Beat(&neptuneConfig, registrationInfo.AgentId)
				if e != nil {
					logging.Error("Could not send heartbeats.", logging.Fields{"error": e})
				}

			case <-uploadLogsTickerCh:
				e := api.UploadLogs(&neptuneConfig, logFilePath, registrationInfo.AgentId)
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

	events := make(chan *api.Event, 10)
	actionOutputs := make(chan *api.ActionOutputMessage, 10)

	// Start a GO routine to process SQS messages in an infinite loop.
	go func() {
		worker.RunLoop(registrationInfo, regInfoUpdatesCh, events, triggerReregistrationCh)
	}()

	// Start a GO routine to execute the runbooks handed over by the SQS worker.
	go func() {
		for event := range events {
			// Execute each action in a separate go routine.
			go func() {
				executor.ExecuteAction(event, registrationInfo, actionOutputs, agentConfig.GithubApiKey)
			}()
		}
	}()

	// Start a GO routine to send the runbook execution results to Neptune.io service.
	go func() {
		for actionOutput := range actionOutputs {
			if e := api.SendActionOutput(&neptuneConfig, actionOutput); e != nil {
				logging.Error("Could not send action output to Neptune.", logging.Fields{"error": e})
			}
		}
	}()

	<-exitChannel

	return nil
}
