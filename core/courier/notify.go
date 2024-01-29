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
	"github.com/pkg/errors"
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

func notifyCourier(baseURL string, channelUUID models.ChannelUUID, payload *courierPayload) error {
	url := fmt.Sprintf("%s/c/twc/%s/receive", baseURL, channelUUID)
	request, _ := httpx.NewRequest("POST", url, bytes.NewReader(jsonx.MustMarshal(payload)), nil)

	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		return errors.Wrap(err, "error connecting courier")
	} else if resp.StatusCode/100 != 2 {
		return errors.New("courier returned non-2XX status")
	}

	slog.Info("courier notified", "event", payload.Type, "status", resp.StatusCode)
	return nil
}

func NotifyChatStarted(cfg *runtime.Config, channel models.Channel, contact *models.Contact) error {
	return notifyCourier(cfg.Courier, channel.UUID(), &courierPayload{
		Type: "chat_started",
		Chat: &courierChat{
			ChatID: contact.ChatID,
		},
	})
}

func NotifyMsgIn(cfg *runtime.Config, channel models.Channel, contact *models.Contact, e *events.MsgIn) error {
	return notifyCourier(cfg.Courier, channel.UUID(), &courierPayload{
		Type: "msg_in",
		Msg: &courierMsg{
			ChatID: contact.ChatID,
			Text:   e.Text,
		},
	})
}
