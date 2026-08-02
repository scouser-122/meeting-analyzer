package config

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"strings"
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

	// MaxUploadSize specifies limit for simultaneous metting processing jobs
	ProcessorLimit *int64 `env:"PROCESSOR_LIMIT" yaml:"processor_limit"`

	// SaluteSpeech specifies config to interact with SaluteSpeech API
	SaluteSpeech *SaluteSpeechConfig `yaml:"salute_speech"`

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
		ProcessorLimit:   new(int64),
		SaluteSpeech:     new(SaluteSpeechConfig),
	}
	*result.RunAddr = "localhost:8080"
	*result.LogLevel = "info"
	*result.Environment = "dev"
	*result.DBDataSourceName = "postgres://postgres:password@localhost:5432/mydb?sslmode=disable"
	*result.ShutdownTimeout = 30 * time.Second
	*result.ProcessorLimit = 10
	return result
}

// Parse parses config from different sources
func (s *ServerConfig) Parse() {
	s.getConfigFileParam()
	s.overrideFromLocalFileIfExists()
	s.parseEnvVariables()
}

func (s *ServerConfig) getConfigFileParam() {
	flag.StringVar(s.ConfigFile, "config", "config.yaml", "config file path")
	flag.Parse()
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != "" {
		*s.ConfigFile = envConfigFile
	}
}

func (s *ServerConfig) parseEnvVariables() {
	err := env.Parse(s)
	if err != nil {
		log.Fatal(err)
	}
}

// overrideFromLocalFileIfExists overrides parameters which were not set by flags or env variables
func (s *ServerConfig) overrideFromLocalFileIfExists() {
	if *s.ConfigFile == "" {
		slog.Info("config file not specified")
		return
	}
	var dirPath string
	lastSlash := strings.LastIndex(*s.ConfigFile, "/")
	if lastSlash >= 0 {
		dirPath = (*s.ConfigFile)[:lastSlash]
	}

	root, err := os.OpenRoot(dirPath)
	if err != nil {
		slog.Error("open config file directory error", "error", err)
		return
	}
	defer root.Close()

	fileName := (*s.ConfigFile)[lastSlash+1:]
	file, err := root.Open(fileName)
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
