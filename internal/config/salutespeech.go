package config

type SaluteSpeechConfig struct {
	// ServerAddress specifies address to interact with SaluteSpeech API
	ServerAddress string `yaml:"server_address"`
	// RetryCount specifies SaluteSpeech API call retry count
	RetryCount int `yaml:"retry_count"`
	// RetryWaitTime specifies SaluteSpeech API call retry wait time in seconds
	RetryWaitTime int `yaml:"retry_wait_time"`
	// RetryMaxTime specifies SaluteSpeech API call retry wait time in seconds
	RetryMaxTime int `yaml:"retry_max_time"`
	// StatusPollingInterval specifies SaluteSpeech API get status call polling interval in seconds
	StatusPollingInterval int `yaml:"status_polling_interval"`
	// StatusPollingMaxAttempts specifies SaluteSpeech API get status call polling max attempts
	StatusPollingMaxAttempts int `yaml:"status_polling_max_attempts"`
	// UploadFileDir specifies directory to upload files
	RecognizedFileDir string `yaml:"recognized_file_dir"`
}
