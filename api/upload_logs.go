// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package api

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/logging"

	"gopkg.in/jmcvetta/napping.v3"
)

const (
	numLinesToReturnFromLogFile = 50
)

// Global variable to hold the last modified time of the agent log file. This is used to avoid uploading
// agent logs to Neptune.io service when it's not necessary.
var logFileModifiedTime int64

// Message sent by Agent to Neptune.io service to upload agent logs.
type UploadLogsRequest struct {
	ErrorMessage string
	AgentId      string
	FullLogs     bool
	Hostname     string
}

func shouldUploadLogs(filename string) bool {

	info, err := os.Stat(filename)
	if err != nil {
		logging.Error("Error opening log file.", logging.Fields{"error": err})
		return false
	}

	// Get the latest modified time of the log file.
	previousModTime := logFileModifiedTime
	logFileModifiedTime = info.ModTime().Unix()

	return (previousModTime == 0 || logFileModifiedTime > previousModTime)
}

// Function to upload agent logs to Neptune.io service.
func UploadLogs(configObj *config.NeptuneConfig, filename string, agentId string) error {
	if !shouldUploadLogs(filename) {
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		logging.Error("Could not open the log file.", logging.Fields{"error": err})
	}

	defer file.Close()

	// Read all the lines from the file.
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		logging.Error("Could not read the log file.", logging.Fields{"error": err})
	}

	// Get the offset of the starting line if we have to send last 50 lines from the file.
	offset := len(lines) - numLinesToReturnFromLogFile
	if offset < 0 {
		offset = 0
	}
	logContent := strings.Join(lines[offset:], "\n")

	logging.Debug("Uploading logs to Neptune.", nil)
	request := UploadLogsRequest{AgentId: agentId, FullLogs: true, ErrorMessage: logContent}
	response := Response{}
	resp, err := napping.Post(joinURL(configObj.Endpoint, "upload_logs", configObj.ApiKey), &request, &response, nil)
	if err != nil {
		logging.Error("Could not post to server.", logging.Fields{"error": err, "response": resp})
		return err
	}

	if 200 <= resp.Status() && resp.Status() <= 299 {
		return nil
	} else {
		return errors.New("Server returned unexpected status: " + strconv.Itoa(resp.Status()))
	}
}
