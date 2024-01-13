package webchat

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/runtime"
)

type courierChat struct {
	Identifier string `json:"identifier"`
}

type courierMsg struct {
	Identifier string `json:"identifier"`
	Text       string `json:"text"`
}

func notifyCourierChatStarted(cfg *runtime.Config, c *Client, e *chatStartedEvent) {
	callCourier(cfg, c.channelUUID, &courierPayload{
		Type: "chat_started",
		Chat: &courierChat{
			Identifier: c.identifier,
		},
	})
}

type courierPayload struct {
	Type string       `json:"type"`
	Chat *courierChat `json:"chat"`
	Msg  *courierMsg  `json:"msg"`
}

func notifyCourierMsgIn(cfg *runtime.Config, c *Client, e *msgInEvent) {
	callCourier(cfg, c.channelUUID, &courierPayload{
		Type: "msg_in",
		Msg: &courierMsg{
			Identifier: c.identifier,
			Text:       e.Text,
		},
	})
}

func callCourier(cfg *runtime.Config, channelUUID uuids.UUID, payload *courierPayload) {
	request, _ := httpx.NewRequest(
		"POST",
		courierURL(cfg, channelUUID),
		bytes.NewReader(jsonx.MustMarshal(payload)), nil,
	)
	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		slog.Error("error connecting to courier", "error", err)
	} else {
		slog.Info("courier notified", "event", payload.Type, "status", resp.StatusCode)
	}
}

func courierURL(cfg *runtime.Config, channelUUID uuids.UUID) string {
	proto := "https"
	if !cfg.CourierSSL {
		proto = "http"
	}
	return fmt.Sprintf("%s://%s/c/twc/%s/receive", proto, cfg.CourierHost, channelUUID)
}
