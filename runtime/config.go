package runtime

import (
	"log"
	"log/slog"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/nyaruka/ezconf"
)

type Config struct {
	Address string `help:"the address to bind our web server to"`
	Port    int    `help:"the port to bind our web server to"`
	Domain  string `help:"the domain that the server is listening on"`
	SSL     bool   `help:"whether server is using SSL"`

	DB         string `validate:"url,startswith=postgres:"           help:"URL for your Postgres database"`
	Valkey     string `validate:"url,startswith=valkey:"             help:"URL for your Valkey instance"`
	StorageURL string `validate:"url"                                help:"URL base for public storage, e.g. avatars"`
	SentryDSN  string `                                              help:"the DSN used for logging errors to Sentry"`

	AWSAccessKeyID     string `help:"access key ID to use for AWS services"`
	AWSSecretAccessKey string `help:"secret access key to use for AWS services"`
	AWSRegion          string `help:"region to use for AWS services, e.g. us-east-1"`

	CloudwatchNamespace string `help:"the namespace to use for cloudwatch metrics"`
	DeploymentID        string `help:"the deployment identifier to use for metrics"`

	InstanceID string     `help:"the unique identifier of this instance, defaults to hostname"`
	LogLevel   slog.Level `help:"the logging level to use"`
	Version    string     `help:"the version of this install"`
}

func NewDefaultConfig() *Config {
	hostname, _ := os.Hostname()

	return &Config{
		Address: "localhost",
		Port:    8070,
		Domain:  "localhost",
		SSL:     false,

		DB:         "postgres://temba:temba@localhost/temba?sslmode=disable&Timezone=UTC",
		Valkey:     "valkey://localhost:6379/5",
		StorageURL: "http://localhost/media/",

		AWSAccessKeyID:     "",
		AWSSecretAccessKey: "",
		AWSRegion:          "us-east-1",

		CloudwatchNamespace: "Temba",
		DeploymentID:        "dev",

		InstanceID: hostname,
		LogLevel:   slog.LevelInfo,
		Version:    "Dev",
	}
}

func LoadConfig() *Config {
	config := NewDefaultConfig()
	loader := ezconf.NewLoader(config, "chip", "Chip - webchat server", []string{"config.toml"})
	loader.MustLoad()

	// ensure config is valid
	if err := config.Validate(); err != nil {
		log.Fatalf("invalid config: %s", err)
	}

	return config
}

// Validate validates the config
func (c *Config) Validate() error {
	return validator.New().Struct(c)
}
