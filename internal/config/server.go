package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v6"
)

// ServerConfig for gophermart service
type ServerConfig struct {
	// RunAddr specifies address and port to run server app
	RunAddr *string `env:"RUN_ADDRESS" json:"run_address"`

	// LogLevel specifies logging level
	LogLevel *string `env:"LOG_LEVEL" json:"log_level"`

	// Environment specifies run environment, dev or prod
	Environment *string `env:"SERVICE_ENVIRONMENT" json:"service_environment"`

	// DBDataSourceName specifies database URI
	DBDataSourceName *string `env:"DATABASE_URI" json:"database_uri"`
}

// DefaultServerConfig specified default config for server app
// which may be redefined by command line args or environrment variables
func DefaultServerConfig() ServerConfig {
	result := ServerConfig{
		RunAddr:          new(string),
		LogLevel:         new(string),
		Environment:      new(string),
		DBDataSourceName: new(string),
	}
	*result.RunAddr = "localhost:8080"
	*result.LogLevel = "info"
	*result.Environment = "dev"
	*result.DBDataSourceName = "postgres://postgres:password@localhost:5432/mydb?sslmode=disable"
	return result
}

// Parse parses config from different sources
func (config *ServerConfig) Parse() {
	config.parseFlags()
	config.parseEnvVariables()
}

func (config *ServerConfig) parseFlags() {
	flag.StringVar(config.RunAddr, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(config.LogLevel, "l", "info", "logging level")
	flag.StringVar(config.Environment, "e", "dev", "environment")
	flag.StringVar(config.DBDataSourceName, "d", "", "URI for database connection")
	flag.Parse()
}

func (config *ServerConfig) parseEnvVariables() {
	err := env.Parse(config)
	if err != nil {
		log.Fatal(err)
	}
}
