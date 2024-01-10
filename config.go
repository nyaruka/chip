package tembachat

type Config struct {
	Address     string `help:"the address to bind our web server to"`
	Port        int    `help:"the port to bind our web server to"`
	CourierHost string `help:"the host name of the courier instance to notify of new messages"`
	CourierSSL  bool   `help:"whether the courier instance uses SSL"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Address:     "localhost",
		Port:        8070,
		CourierHost: "localhost:8080",
		CourierSSL:  false,
	}
}
