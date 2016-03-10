// Package config is responsible for parsing the agent config file, command-line params
// and coming up with the final config.
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// Combine config object for agent. This contains sections for Neptune.io, agent machine data, etc.
type Config struct {
	Neptune NeptuneConfig
	Agent   AgentConfig
}

// Neptune.io section of config file.
type NeptuneConfig struct {
	ApiKey   string
	Endpoint string
}

// Agent (machine info) section of the config file.
type AgentConfig struct {
	AssignedHostname string
	LogFile          string
	DebugMode        bool
	GithubApiKey     string
}

const (
	DefaultBaseURL        = "www.neptune.io"
	DefaultConfigFileName = "neptune-agent.json"
	defaultLogFileName    = "neptune-agent.log"
)

func parseConfig(configFilePath string) (Config, error) {
	file, e := ioutil.ReadFile(configFilePath)
	if e != nil {
		fmt.Printf("Could not read the config file. Error: %v\n", e)
		return Config{}, e
	}

	var obj Config
	e = json.Unmarshal(file, &obj)
	if e != nil {
		fmt.Printf("Could not deserialize the config JSON. Error: %v\n", e)
		return Config{}, e
	}
	return obj, nil
}

func getDefaultConfig() Config {
	return Config{
		NeptuneConfig{Endpoint: DefaultBaseURL},
		AgentConfig{LogFile: defaultLogFileName, DebugMode: false},
	}
}

// Merges the command-line config and the config from file.
func mergeConfigs(cmdConfig NeptuneConfig, configObj Config) (NeptuneConfig, AgentConfig, error) {
	var apiKey string
	if len(cmdConfig.ApiKey) > 0 {
		apiKey = cmdConfig.ApiKey
	} else {
		apiKey = configObj.Neptune.ApiKey
	}

	var endPoint string
	if len(cmdConfig.Endpoint) > 0 {
		endPoint = cmdConfig.Endpoint
	} else {
		endPoint = configObj.Neptune.Endpoint
	}

	return NeptuneConfig{
			ApiKey:   apiKey,
			Endpoint: endPoint,
		},
		configObj.Agent, nil
}

// Function to return config objects based on the config file, command-line values, etc.
func GetConfig(configFilePath string, cmdlineConfig NeptuneConfig, errorChannel chan error) (NeptuneConfig, AgentConfig, error) {
	// Construct a config Object based on the config file/default values.
	var configObject Config
	var e error

	// If the config file is passed as an option, parse the config and verify that
	// the api key exists.
	if len(configFilePath) > 0 {
		configObject, e = parseConfig(configFilePath)
		if e != nil {
			errorChannel <- e
			return NeptuneConfig{}, AgentConfig{}, e
		}
	} else {
		configObject = getDefaultConfig()
	}

	// Merge both the configs by giving higher precedence to the flags.
	return mergeConfigs(cmdlineConfig, configObject)
}
