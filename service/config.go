package service

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// Config parameters
var ServerConfig = Configuration{}
var Reg Registry

var configFile = flag.String("config", "./config.json", "config file")

func LoadConfig() {
	file, e := ioutil.ReadFile(*configFile)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}
	//fmt.Printf("Loaded Config: \n%s\n", string(file))
	json.Unmarshal(file, &ServerConfig)
	fmt.Printf("succeeded to read the config: %s\n", *configFile)

	switch ServerConfig.RegistryType {
	case "zookeeper":
		Reg = &ZooKeeperRegistry{}
	case "etcd":
		Reg = &EtcdRegistry{}
	case "etcdv3":
		Reg = &EtcdV3Registry{}
	case "consul":
		Reg = &ConsulRegistry{}
	default:
		fmt.Printf("unsupported registry: %s\n", ServerConfig.RegistryType)
		os.Exit(2)
	}

	if !strings.HasSuffix(ServerConfig.ServiceBaseURL, "/") {
		ServerConfig.ServiceBaseURL += "/"
	}
	Reg.initRegistry()
}

// Configuration is configuration strcut refects the config.json
type Configuration struct {
	RegistryType   string `json:"registry_type"`
	RegistryURL    string `json:"registry_url"`
	ServiceBaseURL string `json:"service_base_url"`
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	User           string `json:"user,omitempty"`
	Password       string `json:"password,omitempty"`
}

type Registry interface {
	initRegistry()
	FetchServices() []*Service
	DeactivateService(name, address string) error
	ActivateService(name, address string) error
	UpdateMetadata(name, address string, metadata string) error
}

// Service is a service endpoint
type Service struct {
	ID       string
	Name     string
	Address  string
	Metadata string
	State    string
	Group    string
}

