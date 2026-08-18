package config

// NexaraConfig holds configuration parameters for the Nexara API client.
type NexaraConfig struct {
	// ServerAddress specifies address to interact with Nexara API
	ServerAddress string `yaml:"server_address"`
	// ApiToken specifies Nexara API auth token
	ApiToken string `yaml:"api_token"`
}
