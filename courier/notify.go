package courier

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
)

type courierChat struct {
	URN urns.URN `json:"urn"`
}

type courierMsg struct {
	URN  urns.URN `json:"urn"`
	Text string   `json:"text"`
}

type courierPayload struct {
	Type string       `json:"type"`
	Chat *courierChat `json:"chat"`
	Msg  *courierMsg  `json:"msg"`
}

func notifyCourier(baseURL string, channelUUID models.ChannelUUID, payload *courierPayload) {
	url := fmt.Sprintf("%s/c/twc/%s/receive", baseURL, channelUUID)
	request, _ := httpx.NewRequest("POST", url, bytes.NewReader(jsonx.MustMarshal(payload)), nil)

	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		slog.Error("error connecting to courier", "error", err)
	} else {
		slog.Info("courier notified", "event", payload.Type, "status", resp.StatusCode)
	}
}

func Notify(cfg *runtime.Config, c models.Channel, u urns.URN, e events.Event) {
	switch typed := e.(type) {
	case *events.ChatStarted:
		notifyCourier(cfg.Courier, c.UUID(), &courierPayload{
			Type: "chat_started",
			Chat: &courierChat{
				URN: u,
			},
		})

	case *events.MsgIn:
		notifyCourier(cfg.Courier, c.UUID(), &courierPayload{
			Type: "msg_in",
			Msg: &courierMsg{
				URN:  u,
				Text: typed.Text,
			},
		})
	}

}
