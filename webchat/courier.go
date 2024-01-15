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

type courierPayload struct {
	Type string       `json:"type"`
	Chat *courierChat `json:"chat"`
	Msg  *courierMsg  `json:"msg"`
}

func NotifyCourierChatStarted(cfg *runtime.Config, c Client, e *ChatStartedEvent) {
	callCourier(cfg, c.Channel().UUID(), &courierPayload{
		Type: "chat_started",
		Chat: &courierChat{
			Identifier: c.Identifier(),
		},
	})
}

func NotifyCourierMsgIn(cfg *runtime.Config, c Client, e *MsgInEvent) {
	callCourier(cfg, c.Channel().UUID(), &courierPayload{
		Type: "msg_in",
		Msg: &courierMsg{
			Identifier: c.Identifier(),
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
