// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package agent

import (
	"errors"
	"strconv"

	"github.com/neptuneio/agent/logging"

	"gopkg.in/jmcvetta/napping.v3"
)

// Message sent by Agent to Neptune.io service to report runbook execution results.
type ActionOutputMessage struct {
	RuleName         string `json:"ruleName"`
	RuleId           string `json:"ruleId"`
	AgentId          string `json:"agentId"`
	EventId          string `json:"eventId"`
	Status           string `json:"status"`
	ActionOutput     string `json:"actionOutput"`
	FailureReason    string `json:"failureReason"`
	StatusCode       int    `json:"statusCode"`
	InflightActionId string `json:"inflightActionId"`
	IsTimeout        bool   `json:"isTimeout"`
	HostName         string `json:"hostName"`
	ActionType       string `json:"actionType"`
}

// Function to upload runbook execution results to Neptune.io service.
func SendActionOutput(configObj *NeptuneConfig, request *ActionOutputMessage) error {

	logging.Debug("Sending action output to Neptune.", logging.Fields{"request": *request})
	response := Response{}
	resp, err := napping.Post(joinURL(configObj.Endpoint, "action_status", configObj.ApiKey), request, &response, nil)
	if err != nil {
		logging.Warn("Could not post action output to server.", logging.Fields{"error": err, "response": resp})
		return err
	}

	if 200 <= resp.Status() && resp.Status() <= 299 {
		return nil
	} else {
		return errors.New("Server returned unexpected status: " + strconv.Itoa(resp.Status()))
	}
}
