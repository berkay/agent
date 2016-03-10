// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package api

import (
	"errors"
	"strconv"

	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/logging"

	"gopkg.in/jmcvetta/napping.v3"
)

// Message sent by Agent to Neptune.io service as a heartbeat.
type Heartbeat struct {
	Status string
}

// Function to send a heartbeat to Neptune.io service.
func Beat(configObj *config.NeptuneConfig, agentId string) error {
	request := Heartbeat{Status: CurrentStatus().String()}
	response := Response{}

	logging.Debug("Sending heartbeat to Neptune.", logging.Fields{"request": request})
	resp, err := napping.Post(joinURL(configObj.Endpoint, "heartbeat", configObj.ApiKey, agentId), &request, &response, nil)
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
