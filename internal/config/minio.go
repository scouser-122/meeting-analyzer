package config

// MinioConfig for MinIO object storage.
type MinioConfig struct {
	Endpoint  string `env:"MINIO_ENDPOINT" yaml:"endpoint"`
	AccessKey string `env:"MINIO_ACCESS_KEY" yaml:"access_key"`
	SecretKey string `env:"MINIO_SECRET_KEY" yaml:"secret_key"`
	Bucket    string `env:"MINIO_BUCKET" yaml:"bucket"`
	UseSSL    bool   `env:"MINIO_USE_SSL" yaml:"use_ssl"`
	Region    string `env:"MINIO_REGION" yaml:"region"`
	Prefix    string `env:"MINIO_PREFIX" yaml:"prefix"`
}
