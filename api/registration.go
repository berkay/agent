// Package api is responsible for communication between agent and Neptune.io service.
// This includes the data structures and logic related to agent registration, heartbeating,
// uploading runbook execution results, uploading agent errors, etc.
package api

import (
	"errors"
	"strconv"
	"time"

	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/logging"
	"github.com/neptuneio/agent/metadata"

	"gopkg.in/jmcvetta/napping.v3"
)

const (
	// Agent's current version.
	// Note: We should improve the versioning logic to avoid hard-coding like this.
	AgentVersion = "1.1.1"
)

// Message sent by Agent to Neptune.io service to register itself.
type RegistrationRequest struct {
	AgentVersion       string
	Hostname           string
	AssignedHostname   string
	ProviderServerId   string
	ProviderServerType string
	Platform           string
	PrivateIpAddress   string
	PrivateDnsName     string
	PublicIpAddress    string
	PublicDnsName      string
	Region             string
	StartTime          int64
}

// Message received from Neptune.io service when agent's registration has succeeded.
type RegistrationInfo struct {
	AgentId             string
	CreateTime          int64
	UpdateTime          int64
	ActionQueueEndpoint string
	AWSAccessKey        string
	AWSSecretAccessKey  string
	AWSSecurityToken    string
}

// Time at which this agent has started.
var startTime = time.Now().Unix() * 1000

func getAgentRegistrationRequest(data metadata.HostMetaData) RegistrationRequest {
	return RegistrationRequest{
		AgentVersion:       AgentVersion,
		Hostname:           data.HostName,
		AssignedHostname:   data.AssignedHostname,
		ProviderServerId:   data.ProviderId,
		ProviderServerType: data.ProviderType,
		Platform:           data.Platform,
		PrivateIpAddress:   data.PrivateIpAddress,
		PublicIpAddress:    data.PublicIpAddress,
		PrivateDnsName:     data.PrivateDnsName,
		PublicDnsName:      data.PublicDnsName,
		Region:             data.Region,
		StartTime:          startTime,
	}
}

// Function to register this agent with Neptune.io service.
func RegisterAgent(data metadata.HostMetaData, configObj *config.NeptuneConfig) (*RegistrationInfo, error) {
	request := getAgentRegistrationRequest(data)
	response := RegistrationInfo{}
	logging.Info("Registering the agent.", logging.Fields{"request": request})

	resp, err := napping.Post(joinURL(configObj.Endpoint, "register", configObj.ApiKey), &request, &response, nil)
	if err != nil {
		logging.Error("Could not post to server.", logging.Fields{"error": err, "response": resp})
		return &response, err
	}

	if 200 <= resp.Status() && resp.Status() <= 299 {
		logging.Info("Successfully registered the agent.", logging.Fields{"agentId": response.AgentId})
		return &response, nil
	} else {
		logging.Warn("Unexpected status from server.", logging.Fields{"status": resp.Status()})
		return &response, errors.New("Server returned unexpected status: " + strconv.Itoa(resp.Status()))
	}
}
