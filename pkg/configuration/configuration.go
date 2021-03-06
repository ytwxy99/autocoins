package configuration

import (
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

func (c contextKey) String() string {
	return "auth " + string(c)
}

var (
	// ContextPublic takes a boolean value indicating if the request is public
	ContextPublic = contextKey("public")

	// ContextBasicAuth takes BasicAuth as authentication for the request.
	ContextBasicAuth = contextKey("basic")

	// ContextAccessToken takes a string oauth2 access token as authentication for the request.
	ContextAccessToken = contextKey("accesstoken")

	// ContextGateAPIV4 takes a Gate APIv4 key pair as authentication for the request
	ContextGateAPIV4 = contextKey("apiv4")
)

// system configuration
type SystemConf struct {
	TrendCsv            string `yaml:"TrendCsv"`
	CointCsv            string `yaml:"CointCsv"`
	UmbrellaCsv         string `yaml:"UmbrellaCsv"`
	WeightCsv           string `yaml:"WeightCsv"`
	Platform            string `yaml:"PlatformCsv"`
	CointegrationSrcipt string `yaml:"CointegrationSrcipt"`
	LogPath             string `yaml:"LogPath"`
	DBPath              string `yaml:"DBPath"`
	DBType              string `yaml:"DBType"`
	Options             struct {
		Quantity  float32 `yaml:"quantity"`
		Pairing   string  `yaml:"pairing"`
		Test      bool    `yaml:"test"`
		Sl        float32 `yaml:"sl"`
		Tp        float32 `yaml:"tp"`
		EnableTsl bool    `yaml:"enable_tsl"`
		Tsl       float32 `yaml:"tsl"`
		Ttp       float32 `yaml:"ttp"`
	}
	Email struct {
		User     string   `yaml:"user"`
		Password string   `yaml:"password"`
		Host     string   `yaml:"host"`
		Port     string   `yaml:"port"`
		MailTo   []string `yaml:"mailTo"`
	}
	Mysql struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Database string `yaml:"Database"`
	}
}

// BasicAuth provides basic http authentication to a request passed via context using ContextBasicAuth
type BasicAuth struct {
	UserName string `json:"userName,omitempty"`
	Password string `json:"password,omitempty"`
}

// APIKey provides API key based authentication to a request passed via context using ContextAPIKey
type APIKey struct {
	Key    string
	Prefix string
}

// GateAPIV4 provides Gate APIv4 based authentication to a request passed via context using ContextGateAPIV4
type GateAPIV4 struct {
	Key    string `yaml:"Key"`
	Secret string `yaml:"Secret"`
}

// ServerVariable stores the information about a server variable
type ServerVariable struct {
	Description  string
	DefaultValue string
	EnumValues   []string
}

// ServerConfiguration stores the information about a server
type ServerConfiguration struct {
	Url         string
	Description string
	Variables   map[string]ServerVariable
}

// Configuration stores the configuration of the API client
type Configuration struct {
	BasePath      string            `json:"basePath,omitempty"`
	Host          string            `json:"host,omitempty"`
	Scheme        string            `json:"scheme,omitempty"`
	DefaultHeader map[string]string `json:"defaultHeader,omitempty"`
	UserAgent     string            `json:"userAgent,omitempty"`
	Key           string            `json:"key,omitempty"`
	Secret        string            `json:"secret,omitempty"`
	Debug         bool              `json:"debug,omitempty"`
	Servers       []ServerConfiguration
	HTTPClient    *http.Client
}

// NewConfiguration returns a new Configuration object
func NewConfiguration() *Configuration {
	cfg := &Configuration{
		BasePath:      "https://api.gateio.ws/api/v4",
		DefaultHeader: make(map[string]string),
		UserAgent:     "OpenAPI-Generator/6.22.4/go",
		Debug:         false,
		Servers: []ServerConfiguration{
			{
				Url:         "https://api.gateio.ws/api/v4",
				Description: "Real Trading",
			},
			{
				Url:         "https://fx-api-testnet.gateio.ws/api/v4",
				Description: "TestNet Trading",
			},
		},
	}
	return cfg
}

// AddDefaultHeader adds a new HTTP header to the default header in the request
func (c *Configuration) AddDefaultHeader(key string, value string) {
	c.DefaultHeader[key] = value
}

// ServerUrl returns URL based on server settings
func (c *Configuration) ServerUrl(index int, variables map[string]string) (string, error) {
	if index < 0 || len(c.Servers) <= index {
		return "", fmt.Errorf("Index %v out of range %v", index, len(c.Servers)-1)
	}
	server := c.Servers[index]
	url := server.Url

	// go through variables and replace placeholders
	for name, variable := range server.Variables {
		if value, ok := variables[name]; ok {
			found := bool(len(variable.EnumValues) == 0)
			for _, enumValue := range variable.EnumValues {
				if value == enumValue {
					found = true
				}
			}
			if !found {
				return "", fmt.Errorf("The variable %s in the server URL has invalid value %v. Must be %v", name, value, variable.EnumValues)
			}
			url = strings.Replace(url, "{"+name+"}", value, -1)
		} else {
			url = strings.Replace(url, "{"+name+"}", variable.DefaultValue, -1)
		}
	}
	return url, nil
}
