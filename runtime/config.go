package runtime

import (
	"log"
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/nyaruka/ezconf"
)

type Config struct {
	Address   string `help:"the address to bind our web server to"`
	Port      int    `help:"the port to bind our web server to"`
	Courier   string `help:"the base URL of the courier instance to notify of events"`
	DB        string `validate:"url,startswith=postgres:"           help:"URL for your Postgres database"`
	Redis     string `validate:"url,startswith=redis:"              help:"URL for your Redis instance"`
	SentryDSN string `                                              help:"the DSN used for logging errors to Sentry"`

	LogLevel slog.Level `help:"the logging level to use"`
	Version  string     `help:"the version of this install"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Address: "localhost",
		Port:    8070,
		Courier: "http://localhost:8080",

		LogLevel: slog.LevelInfo,
		Version:  "Dev",
	}
}

func LoadConfig() *Config {
	config := NewDefaultConfig()
	loader := ezconf.NewLoader(config, "chatserver", "Temba Chat - webchat server", []string{"config.toml"})
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
