package courier

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
)

type courierChat struct {
	ChatID models.ChatID `json:"chat_id"`
}

type courierMsg struct {
	ChatID models.ChatID `json:"chat_id"`
	Text   string        `json:"text"`
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

func Notify(cfg *runtime.Config, channel models.Channel, contact models.Contact, e events.Event) {
	switch typed := e.(type) {
	case *events.ChatStarted:
		notifyCourier(cfg.Courier, channel.UUID(), &courierPayload{
			Type: "chat_started",
			Chat: &courierChat{
				ChatID: contact.ChatID(),
			},
		})

	case *events.MsgIn:
		notifyCourier(cfg.Courier, channel.UUID(), &courierPayload{
			Type: "msg_in",
			Msg: &courierMsg{
				ChatID: contact.ChatID(),
				Text:   typed.Text,
			},
		})
	}

}
