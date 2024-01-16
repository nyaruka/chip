package runtime

import "log/slog"

type Config struct {
	Address string `help:"the address to bind our web server to"`
	Port    int    `help:"the port to bind our web server to"`
	Courier string `help:"the base URL of the courier instance to notify of events"`

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
