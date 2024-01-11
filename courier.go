package webchat

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
)

type courierMessage struct {
	Identifier string `json:"identifier"`
	Text       string `json:"text"`
}

type courierPayload struct {
	Type    string          `json:"type"`
	Message *courierMessage `json:"message"`
}

func notifyCourier(cfg *Config, c *Client, e *msgInEvent) {
	courierBody := &courierPayload{
		Type: "message",
		Message: &courierMessage{
			Identifier: c.identifier,
			Text:       e.Text,
		},
	}

	courierProto := "https"
	if !cfg.CourierSSL {
		courierProto = "http"
	}
	courierURL := fmt.Sprintf("%s://%s/c/twc/%s/receive", courierProto, cfg.CourierHost, c.channelUUID)
	request, _ := httpx.NewRequest("POST", courierURL, bytes.NewReader(jsonx.MustMarshal(courierBody)), nil)

	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		slog.Error("error connecting to courier", "error", err)
	} else {
		slog.Info("courier notified of new message", "status", resp.StatusCode)
	}
}
