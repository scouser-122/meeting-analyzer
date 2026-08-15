package config

type GigaChatConfig struct {
	// GetTokenAddress specifies address to get token
	GetTokenAddress string `yaml:"get_token_address"`
	// AuthKey specifies auth key
	AuthKey string `yaml:"auth_key"`
	// ServerAddress specifies address to get make API requests
	ServerAddress string `yaml:"server_address"`
	// Model specifies model name
	Model string `yaml:"model"`
}
