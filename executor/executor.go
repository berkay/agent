// Package executor is responsible for executing runbooks on agent machine.
// This will have logic to fetch Github runbooks if the agent is configured to use
// Github.
package executor

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/neptuneio/agent/api"
	"github.com/neptuneio/agent/logging"
	"github.com/neptuneio/agent/state"
	"github.com/neptuneio/agent/worker"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

const (
	// Currently we truncate the runbook execution output to 10MB.
	maxActionOutputSize = 10 * 1024 * 1024 // 10MB
	filePathSeparator   = "/"

	// All the events/SQS messages older than 10min are discarded as stale.
	stalenessTimeout = 10 * 60 * 1000
)

var workingDir string

func init() {
	// Get the full path of the binary and there by the working directory.
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		workingDir = dir
	} else {
		// This is a fallback case.
		workingDir = "."
	}
}

// Function to fetch a particular runbook from Github. Currently, agent uses read-only personal access token
// to fetch the runbooks. In future we might add support to fetch the entire repo using deploy keys, if need be.
func getRunbookFromGithub(token, fullPath string) (string, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	parts := strings.Split(fullPath, filePathSeparator)
	if len(parts) < 3 {
		logging.Error("Github runbook path does not have required fields.", logging.Fields{"path": fullPath})
		return "", errors.New("Incomplete github runbook path.")
	}

	logging.Debug("Getting runbook from Github.", logging.Fields{"path": fullPath})
	runbook, err := client.Repositories.DownloadContents(parts[0], parts[1], strings.Join(parts[2:], filePathSeparator), nil)
	if err != nil {
		logging.Error("Could not download runbook from Github.", logging.Fields{"error": err})
		return "", err
	}
	defer runbook.Close()

	bytes, err := ioutil.ReadAll(runbook)
	if err != nil {
		logging.Error("Error reading runbook body", logging.Fields{"error": err})
		return "", err
	}

	return string(bytes), nil
}

// Function to write the runbook to a temp file.
func writeToTmpFile(eventId, runbookName string, rawCmd *string) (string, error) {
	var extension string
	if runtime.GOOS == "windows" {
		if len(runbookName) > 0 && strings.HasSuffix(runbookName, ".ps1") {
			extension = ".ps1"
		} else {
			extension = ".cmd"
		}
	} else {
		extension = ".sh"
	}
	fileName, err := filepath.Abs(filepath.Join(workingDir, strings.Join([]string{eventId, extension}, "")))
	if err != nil {
		logging.Warn("Could not get absolute path of the file.", logging.Fields{"error": err, "file": fileName})
	}

	f, err := os.Create(fileName)
	if err != nil {
		logging.Error("Could not create tmp file", logging.Fields{"error": err})
		return fileName, err
	} else {
		// Make the file executable.
		os.Chmod(fileName, os.ModePerm)
		defer f.Close()
	}

	_, err = f.WriteString(*rawCmd)
	if err != nil {
		logging.Error("Could not write the commands to temp file.", logging.Fields{"error": err, "file": fileName})
		return "", err
	} else {
		f.Sync()
	}

	return fileName, nil
}

func sendActionOutput(regInfo *api.RegistrationInfo, actionOutputs chan<- *api.ActionOutputMessage,
	event *api.Event, stdout, stderr string, status string, statusCode int, timeout bool) error {
	actionOutputs <- &api.ActionOutputMessage{
		RuleName:         event.RuleName,
		RuleId:           event.RuleId,
		HostName:         event.Hostname,
		EventId:          event.EventId,
		InflightActionId: event.InflightActionId,
		ActionType:       event.ActionType,
		AgentId:          regInfo.AgentId,
		StatusCode:       statusCode,
		Status:           status,
		IsTimeout:        timeout,
		ActionOutput:     stdout,
		FailureReason:    stderr,
	}

	logging.Info("Finished processing the event.", logging.Fields{"eventId": event.EventId,
		"status":   status,
		"exitCode": statusCode,
		"timeout":  timeout})
	return nil
}

// Function to execute the runbook in the given temp file.
func execute(regInfo *api.RegistrationInfo, event *api.Event, tmpFile string) (string, int, bool, string, string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if strings.HasSuffix(tmpFile, ".ps1") {
			cmd = exec.Command("powershell", tmpFile)
		} else {
			cmd = exec.Command(tmpFile)
		}
	} else {
		cmd = exec.Command("/bin/sh", "-c", tmpFile)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	SetPGroup(cmd)

	// Set the environment variables.
	if event.Environment != nil {
		env := os.Environ()
		for k, v := range event.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	status := "SUCCESS"
	timeout := false
	statusCode := 1
	var waitStatus syscall.WaitStatus

	// Start the command first.
	exitError := cmd.Start()

	// Immediately delete the SQS message since the command has started.
	worker.DeleteMessage(regInfo, &event.ReceiptHandle)

	if exitError != nil {
		logging.Error("Could not start the command.", logging.Fields{"error": exitError})
	} else {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// Start a timer to kill the command after given timeout.
		select {
		case <-time.After(time.Second * time.Duration(event.Timeout)):
			logging.Debug("Killing the command.", logging.Fields{"eventId": event.EventId})

			// Kill the command and all its children.
			KillCommand(cmd)

			exitError = <-done // allow goroutine to exit
			timeout = true
			status = "TIMEOUT"
			logging.Info("Killed the command after timeout.", logging.Fields{"error": exitError, "eventId": event.EventId})

		case exitError = <-done:
		}
	}

	if exitError != nil {
		logging.Error("Failed to run the command.", logging.Fields{"error": exitError, "cmdFile": tmpFile})

		status = "FAILED"

		// Did the command fail because of an unsuccessful exit code
		if e, ok := exitError.(*exec.ExitError); ok {
			waitStatus = e.Sys().(syscall.WaitStatus)
			statusCode = waitStatus.ExitStatus()
		} else {
			statusCode = 1
		}
	} else {
		// Command was successful
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		statusCode = waitStatus.ExitStatus()
	}

	return status, statusCode, timeout, stdout.String(), stderr.String()
}

// Main function to execute runbook based on the given event.
//
// This function does following checks before executing the runbook.
// 1. Using persistent event store, it verifies that the newly received event is not a duplicate.
// 2. Based on the timestamp on event, it checks if the event is not too old.
// 3. If the agent is configured to execute only Github runbooks, it double checks that the event contains
//    Github runbook link and agent configuration has the Github access key.
// The event will be discarded and SQS message will be deleted if any of the above checks fail.
func ExecuteAction(event *api.Event, regInfo *api.RegistrationInfo, actionOutputs chan<- *api.ActionOutputMessage, githubKey string) error {

	// Check if this event was already processed. This guards against duplicate events, just in case.
	if state.HasProcessedEvent(event.EventId) {
		logging.Info("Discarding the event since it was already processed.", logging.Fields{"eventId": event.EventId})

		// Delete this event from SQS.
		worker.DeleteMessage(regInfo, &event.ReceiptHandle)
		return nil
	}

	// Check if the event is stale and discard it if so.
	currentMillis := time.Now().UnixNano() / 1000000
	if currentMillis-event.Timestamp > stalenessTimeout {
		logging.Error("Received a stale event. Dropping and deleting it from SQS.",
			logging.Fields{"eventId": event.EventId, "timestamp": event.Timestamp})

		// Delete this event from SQS.
		worker.DeleteMessage(regInfo, &event.ReceiptHandle)
		return nil
	}

	// Check if this agent is configured to run only Github runbooks. If so, discard any other
	// event containing Neptune runbook.
	if len(githubKey) > 0 && len(event.RawCommand) > 0 {
		logging.Error("Agent is configured to run Github runbooks only but received Neptune runbook."+
			" Dropping and deleting the event.", logging.Fields{"eventId": event.EventId})
		// Delete this event from SQS.
		worker.DeleteMessage(regInfo, &event.ReceiptHandle)
		return nil
	}

	// All good to go. Process the event further.
	logging.Info("Processing event.", logging.Fields{"eventId": event.EventId})
	logging.Debug("Event data..", logging.Fields{"event": event})

	var runbookContent *string
	if len(event.GithubFilePath) > 0 {
		if githubKey == "" {
			logging.Error("Github api key or file path is empty.", nil)
			return errors.New("Empty Github api key.")
		} else {
			content, err := getRunbookFromGithub(githubKey, event.GithubFilePath)
			if err != nil {
				return err
			} else {
				runbookContent = &content
			}
		}
	} else {
		runbookContent = &event.RawCommand
	}

	tmpFile, e := writeToTmpFile(event.EventId, event.RunbookName, runbookContent)
	if e != nil {
		return errors.New("Could not write the commands to a file.")
	}
	defer os.Remove(tmpFile)

	// Persist the event so that we don't rerun the action for this event again.
	if err := state.PersistEvent(event); err != nil {
		logging.Error("Could not persist the event.", logging.Fields{"error": err})
	}

	// Execute the command and delete the SQS message after starting the command successfully.
	status, code, timeout, stdout, stderr := execute(regInfo, event, tmpFile)

	// Truncate the stderr and stdout to a maximum value.
	if len(stdout) > maxActionOutputSize {
		stdout = stdout[:maxActionOutputSize-1]
	}

	if len(stderr) > maxActionOutputSize {
		stderr = stderr[:maxActionOutputSize-1]
	}

	e = sendActionOutput(regInfo, actionOutputs, event, stdout, stderr, status, code, timeout)
	if e != nil {
		logging.Error("Could not queue the action output for Neptune", logging.Fields{"error": e})
	} else {
		api.UpdateStatus(api.Active)
	}

	return e
}
