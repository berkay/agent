// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package api

import (
	"strings"
	"sync"
)

// Agent status type definition.
type Status int

const (
	agentApi = "/api/v1/agent/"
	protocol = "https://"
	slash    = "/"

	ConfigReadFailed      Status = 1
	ConfigReadSucceeded   Status = 2
	RegistrationFailed    Status = 4
	RegistrationSucceeded Status = 8
	QueuePollingSucceeded Status = 16
	Active                Status = 32
)

// Response sent by Neptune.io service when agent sends different requests.
type Response struct {
	message string
}

// Event is a type holding the data sent from Neptune.io as a single SQS message.
// Each event corresponds to one execute runbook request to agent.
type Event struct {
	Timestamp        int64             `json:"timestamp"`
	Source           string            `json:"source"`
	Hostname         string            `json:"hostname"`
	ActionType       string            `json:"actionType"`
	EventId          string            `json:"eventId"`
	AgentId          string            `json:"agentId"`
	RuleId           string            `json:"ruleId"`
	RuleName         string            `json:"ruleName"`
	InflightActionId string            `json:"inflightActionId"`
	RunbookName      string            `json:"runbookName"`
	RawCommand       string            `json:"rawCommand"`
	Signature        string            `json:"signature"`
	Timeout          int32             `json:"timeout"`
	GithubFilePath   string            `json:"githubFilePath"`
	Environment      map[string]string `json:"env"`
	SQSMessageId     string
	ReceiptHandle    string
}

// Function to return string representation of Status.
func (t Status) String() string {
	s := ""
	if t&ConfigReadFailed == ConfigReadFailed {
		s += "CONFIG_READ_FAILED"
	} else if t&ConfigReadSucceeded == ConfigReadSucceeded {
		s += "CONFIG_READ_SUCCESS"
	} else if t&RegistrationFailed == RegistrationFailed {
		s += "REGISTRATION_FAILED"
	} else if t&RegistrationSucceeded == RegistrationSucceeded {
		s += "REGISTRATION_SUCCESS"
	} else if t&QueuePollingSucceeded == QueuePollingSucceeded {
		s += "QUEUE_READ_SUCCESS"
	} else if t&Active == Active {
		s += "ACTIVE"
	}

	return s
}

// Global variable to hold current agent status.
var status Status

// Lock used to update the agent status in a thread-safe manner.
var statusLock sync.Mutex

// Function to get current status of the running agent.
func CurrentStatus() Status {
	statusLock.Lock()
	defer statusLock.Unlock()
	return status
}

// Function to update the agent status with the given new status.
func UpdateStatus(newStatus Status) {
	statusLock.Lock()
	if !(status == Active && newStatus == QueuePollingSucceeded) {
		status = newStatus
	}

	statusLock.Unlock()
}

// Helper function to construct Neptune API url.
func joinURL(endpoint string, args ...string) string {
	var trimmedArgs []string
	for _, arg := range args {
		trimmedArgs = append(trimmedArgs, strings.Trim(arg, slash))
	}

	return strings.Join([]string{protocol, strings.TrimRight(endpoint, slash), agentApi, strings.Join(trimmedArgs[:len(trimmedArgs)], slash)}, "")
}
