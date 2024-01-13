package runtime

type Config struct {
	Address     string `help:"the address to bind our web server to"`
	Port        int    `help:"the port to bind our web server to"`
	Redis       string `help:"URL for your Redis instance"              validate:"url,startswith=redis:"`
	CourierHost string `help:"the host name of the courier instance to notify of new messages"`
	CourierSSL  bool   `help:"whether the courier instance uses SSL"`

	LogLevel string `help:"the logging level to use"`
	Version  string `help:"the version of this install"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Address:     "localhost",
		Port:        8070,
		Redis:       "redis://localhost:6379/15",
		CourierHost: "localhost:8080",
		CourierSSL:  false,

		LogLevel: "info",
		Version:  "Dev",
	}
}
