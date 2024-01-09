package tembachat

type Config struct {
	Address string `help:"the address to bind our web server to"`
	Port    int    `help:"the port to bind our web server to"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Address: "localhost",
		Port:    8070,
	}
}
