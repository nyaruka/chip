package runtime

import (
	"log/slog"

	"github.com/nyaruka/chip/v26/utils"
)

// Config is our top level configuration object
type Config struct {
	DB     string `validate:"url,startswith=postgres:"                    help:"URL for the platform's Postgres database"`
	Valkey string `validate:"url,startswith=valkey:|startswith=valkeys:" help:"URL for the Valkey instance, valkeys:// for TLS"`

	Address         string `help:"the address the public listener will bind to, empty means all interfaces"`
	Port            int    `help:"the port the public listener will listen on"`
	InternalAddress string `help:"the address the internal listener will bind to, empty means all interfaces"`
	InternalPort    int    `help:"the port the internal listener will listen on"`

	CourierURL       string `validate:"url" help:"the base URL of courier, whose webchat channel endpoints visitor messages and statuses are posted to"`
	CourierAuthToken string `help:"the authentication token sent on requests to courier"`

	CentrifugoEndpoint string `validate:"url" help:"the endpoint of the Centrifugo server's API"`
	CentrifugoKey      string `help:"the API key for the Centrifugo server"`

	S3Endpoint          string `help:"S3 service endpoint, e.g. https://s3.amazonaws.com"`
	S3AttachmentsBucket string `help:"S3 bucket visitor uploads are written to, as attachments of their messages"`
	S3PublicBucket      string `help:"S3 bucket the platform keeps public files such as user avatars in"`
	S3PathStyle         bool   `help:"S3 should use path style URLs"`

	AuthToken string `help:"the authentication token courier and the realtime server send on requests to the internal listener"`

	DeploymentID string     `help:"the deployment identifier to use for metrics"`
	LogLevel     slog.Level `help:"the logging level chip should use"`
	Version      string     `help:"the version that will be used in request and response headers"`
}

// NewDefaultConfig returns a new default configuration object
func NewDefaultConfig() *Config {
	return &Config{
		DB:     "postgres://temba:temba@postgres/temba?sslmode=disable",
		Valkey: "valkey://valkey:6379/15",

		Address:         "",
		Port:            8070,
		InternalAddress: "",
		InternalPort:    8071,

		CourierURL: "http://courier:8080",

		CentrifugoEndpoint: "http://centrifugo:9000/ws/api",

		S3Endpoint:          "https://s3.amazonaws.com",
		S3AttachmentsBucket: "temba-attachments",
		S3PublicBucket:      "temba-default",
		S3PathStyle:         false,

		DeploymentID: "dev",
		LogLevel:     slog.LevelWarn,
		Version:      "Dev",
	}
}

// Parse validates the config. It's called by cmd.LoadConfig, and a config built by other means (e.g. NewDefaultConfig
// in a test) should be parsed before being handed to NewRuntime.
func (c *Config) Parse() error {
	return utils.Validate(c)
}
