package config

// DBConnectionConfig specifies config for DB interaction
type DBConnectionConfig struct {
	DSN         string
	RetryConfig RetryConfig
}
