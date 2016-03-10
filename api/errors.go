// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package api

import (
	"os"

	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/metadata"

	"gopkg.in/jmcvetta/napping.v3"
)

// Channel to hold agent errors. All components of agents should push errors into this
// channel and a separate thread uploads them to Neptune.io service currently. In future,
// we can even log these to syslog so that customers will catch agent issues sooner.
var ErrorsChannel = make(chan string, 10)

var (
	regInfo       *RegistrationInfo
	md            *metadata.HostMetaData
	neptuneConfig *config.NeptuneConfig
	hostname      string
)

// Data structure to hold an error that has happened on Agent.
// All agent errors are sent to Neptune.io service for quick identification of agent problems.
type AgentError struct {
	ErrorMessage string
	AgentId      string
	FullLogs     bool
	Hostname     string
	Status       string
}

func init() {
	// Grab the host name.
	hostname, _ = os.Hostname()

	// Start a GO routine to upload all agent errors to Neptune.io service.
	go func() {
		for msg := range ErrorsChannel {
			uploadError(msg)
		}
	}()
}

func SetRegistrationInfo(reg *RegistrationInfo, metaData metadata.HostMetaData, nConfig config.NeptuneConfig) {
	regInfo = reg
	md = &metaData
	neptuneConfig = &nConfig
}

// Function to report an error happened on this agent. This function only pushes the error into a channel.
func ReportError(err string) {
	ErrorsChannel <- err
}

// Function to upload an error that happened on this agent to Neptune.io service.
func uploadError(msg string) {
	request := AgentError{ErrorMessage: msg, FullLogs: false, Hostname: hostname, Status: CurrentStatus().String()}
	response := Response{}

	if md != nil {
		request.Hostname = md.HostName
	}

	if regInfo != nil {
		request.AgentId = regInfo.AgentId
	}

	if neptuneConfig != nil {
		_, _ = napping.Post(joinURL(neptuneConfig.Endpoint, "upload_logs", neptuneConfig.ApiKey), &request, &response, nil)
	} else {
		_, _ = napping.Post(joinURL(config.DefaultBaseURL, "upload_logs", ""), &request, &response, nil)
	}
}
