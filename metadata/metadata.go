// Package metadata contains all the meta data used by the agent in registration protocol
package metadata

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/neptuneio/agent/config"
	"github.com/neptuneio/agent/logging"
)

// Host meta data used by the agent in registration protocol.
type HostMetaData struct {
	HostName         string
	AssignedHostname string
	ProviderId       string
	ProviderType     string
	Platform         string
	PrivateIpAddress string
	PrivateDnsName   string
	PublicIpAddress  string
	PublicDnsName    string
	Region           string
}

// Helper function to query the given URL and give back the response.
func queryData(url string) (string, error) {
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	logging.Debug("Querying the url for cloud metadata.", logging.Fields{"url": url})
	resp, err := client.Get(url)
	if err != nil {
		logging.Warn("Could not query cloud metadata.", logging.Fields{
			"url":      url,
			"error":    err,
			"response": resp,
		})
		return "", err
	}

	if 200 <= resp.StatusCode && resp.StatusCode <= 299 {
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logging.Warn("Could not read response.", logging.Fields{"url": url, "error": err})
			return "", errors.New("Could not read response.")
		}
		return string(contents), nil
	} else {
		return "", errors.New("Server returned unexpected status: " + strconv.Itoa(resp.StatusCode))
	}
}

// Returns the non loopback local IP of the host
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// Function to get the complete meta data for the host on which agent is running.
// This method tries to get cloud specific meta data also, in case the machine is in a cloud.
func GetHostMetaData(agentConfig *config.AgentConfig) (HostMetaData, error) {
	logging.Debug("Getting host metadata.", nil)

	hostname, e := os.Hostname()
	if e != nil {
		logging.Error("Could not get host name.", logging.Fields{"error": e})
		os.Exit(1)
	}

	privateIp := getLocalIP()
	publicIp, e := queryData("http://ip.42.pl/raw")
	platform := string(runtime.GOOS) + " " + string(runtime.GOARCH)

	var privateDns string
	if addr, err := net.LookupAddr(privateIp); err == nil && len(addr) > 0 {
		privateDns = addr[0]
	} else {
		logging.Warn("Could not get private DNS name.", logging.Fields{"error": err})
	}

	var publicDns string
	if addr, err := net.LookupAddr(publicIp); err == nil && len(addr) > 0 {
		publicDns = addr[0]
	} else {
		logging.Warn("Could not get public DNS name.", logging.Fields{"error": err})
	}

	var providerServerId string
	var providerType string
	var region string
	providerServerId, e = queryData("http://169.254.169.254/latest/meta-data/instance-id")
	if e == nil && len(providerServerId) > 0 {
		providerType = "AWS"
		regionValue, e := queryData("http://169.254.169.254/latest/meta-data/placement/availability-zone")
		if e != nil && len(regionValue) > 0 {
			region = regionValue
		}
	} else {
		providerType = "NON_AWS"
	}

	data := HostMetaData{
		HostName:         hostname,
		AssignedHostname: agentConfig.AssignedHostname,
		PrivateIpAddress: privateIp,
		PublicIpAddress:  publicIp,
		Platform:         platform,
		PrivateDnsName:   privateDns,
		PublicDnsName:    publicDns,
		ProviderId:       providerServerId,
		ProviderType:     providerType,
		Region:           region,
	}
	return data, nil
}
