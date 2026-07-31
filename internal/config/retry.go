package config

// RetryConfig is configuration parameters for implementing retry logic for any operation
type RetryConfig struct {
	MaxAttempts       int
	InitialBackoff    int
	BackoffMultiplier int
}

// DefaultRetryConfig is a default retry configuration parameters
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    1000,
		BackoffMultiplier: 1000,
	}
}
