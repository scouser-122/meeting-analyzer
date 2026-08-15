package config

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/caarlos0/env/v6"
	"gopkg.in/yaml.v3"
)

// ServerConfig for gophermart service
type ServerConfig struct {
	// RunAddr specifies address and port to run server app
	RunAddr *string `env:"RUN_ADDRESS" yaml:"run_address"`

	// LogLevel specifies logging level
	LogLevel *string `env:"LOG_LEVEL" yaml:"log_level"`

	// Environment specifies run environment, dev or prod
	Environment *string `env:"SERVICE_ENVIRONMENT" yaml:"service_environment"`

	// DBDataSourceName specifies database URI
	DBDataSourceName *string `env:"DATABASE_URI" yaml:"database_uri"`

	// ShutdownTimeout timeout which server will wait to finist processing requests before shutdown
	ShutdownTimeout *time.Duration `env:"SHUTDOW_TIMEOUT" yaml:"shutdow_timeout"`

	// UploadFileDir specifies directory to upload files
	UploadFileDir *string `env:"UPLOAD_FILE_DIR" yaml:"upload_file_dir"`

	// MaxUploadSize specifies max meeting audio file upload size
	MaxUploadSize *int64 `env:"MAX_UPLOAD_SIZE" yaml:"max_upload_size"`

	// ProcessorLimit specifies limit for simultaneous metting processing jobs
	ProcessorLimit *int `env:"PROCESSOR_LIMIT" yaml:"processor_limit"`

	// ProcessorTimeout specifies timeout for metting processing job (in seconds)
	ProcessorTimeout *int `env:"PROCESSOR_TIMEOUT" yaml:"processor_timeout"`

	// SaluteSpeech specifies config to interact with SaluteSpeech API
	SaluteSpeech *SaluteSpeechConfig `yaml:"salute_speech"`

	// GigaChat specifies config to interact with GigaChat API
	GigaChat *GigaChatConfig `yaml:"giga_chat"`

	// ConfigFile config filein yaml format path
	ConfigFile *string
}

// DefaultServerConfig specified default config for server app
// which may be redefined by command line args or environrment variables
func DefaultServerConfig() ServerConfig {
	result := ServerConfig{
		RunAddr:          new(string),
		LogLevel:         new(string),
		Environment:      new(string),
		DBDataSourceName: new(string),
		ShutdownTimeout:  new(time.Duration),
		ConfigFile:       new(string),
		ProcessorLimit:   new(int),
		ProcessorTimeout: new(int),
		SaluteSpeech:     new(SaluteSpeechConfig),
		GigaChat:         new(GigaChatConfig),
	}
	*result.RunAddr = "localhost:8080"
	*result.LogLevel = "info"
	*result.Environment = "dev"
	*result.DBDataSourceName = "postgres://postgres:password@localhost:5432/mydb?sslmode=disable"
	*result.ShutdownTimeout = 30 * time.Second
	*result.ProcessorLimit = 5
	*result.ProcessorTimeout = 30
	return result
}

// Parse parses config from different sources
func (s *ServerConfig) Parse() error {
	s.getConfigFileParam()
	s.overrideFromLocalFileIfExists()
	if err := s.parseEnvVariables(); err != nil {
		return err
	}
	return nil
}

func (s *ServerConfig) getConfigFileParam() {
	flag.StringVar(s.ConfigFile, "config", "config.yaml", "config file path")
	flag.Parse()
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != "" {
		*s.ConfigFile = envConfigFile
	}
}

func (s *ServerConfig) parseEnvVariables() error {
	err := env.Parse(s)
	if err != nil {
		return err
	}
	return nil
}

// overrideFromLocalFileIfExists overrides parameters which were not set by flags or env variables
func (s *ServerConfig) overrideFromLocalFileIfExists() {
	if *s.ConfigFile == "" {
		slog.Info("config file not specified")
		return
	}

	file, err := os.Open(*s.ConfigFile)
	if err != nil {
		slog.Error("open config file error", "error", err)
		return
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(s)
	if err != nil {
		slog.Error("read config file error", "error", err)
		return
	}

	slog.Info("successfully loaded config file", "file", *s.ConfigFile)
}
