package config

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

type Configuration struct {
	XMLName xml.Name `xml:"configuration"`
	App     struct {
		Name string `xml:"name"`
	} `xml:"app"`
	Servers struct {
		Server []struct {
			Name             string `xml:"name"`
			IP               string `xml:"IP"`
			Port             int    `xml:"port"`
			Pattern          string `xml:"pattern"`
			IsNumberResponse bool   `xml:"isNumberResponse"`
		} `xml:"server"`
	} `xml:"servers"`
}

type LoginServer struct {
	Name             string
	IP               string
	Port             int
	Pattern          []byte
	IsNumberResponse bool
}

func hexStringToBytes(hexStr string) ([]byte, error) {
	// remove "0x" prefix if present
	hexStr = strings.TrimPrefix(hexStr, "0x")
	// if the length of the hex string is odd, prepend a "0"
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	// decode the hex string to bytes
	return hex.DecodeString(hexStr)
}

func LoadCustomServersFromXML(path string) ([]LoginServer, error) {
	// check if the config.xml file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// config.xml not found, return nil
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error checking config.xml at %s: %v", path, err)
	}

	// read the config.xml file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.xml at %s: %v", path, err)
	}

	// parse XML
	var config Configuration
	err = xml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config.xml at %s: %v", path, err)
	}

	// check if the App.Name matches
	if config.App.Name != "ro-server-player-monitor" {
		// not match, ignore the config
		return nil, nil
	}

	// convert the `pattern` field from hex string to byte array
	loginServers := make([]LoginServer, len(config.Servers.Server))

	fmt.Println("[LoadCustomServersFromXML] Found custom servers:", len(config.Servers.Server))

	for i, server := range config.Servers.Server {
		pattern, err := hexStringToBytes(server.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to convert pattern from hex string to bytes: %v", err)
		}
		loginServers[i] = LoginServer{
			Name:             server.Name,
			IP:               server.IP,
			Port:             server.Port,
			Pattern:          pattern,
			IsNumberResponse: server.IsNumberResponse,
		}
	}

	return loginServers, nil
}
