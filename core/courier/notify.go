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

type receivePayload struct {
	ChatID models.ChatID `json:"chat_id"`
	Secret string        `json:"secret"`
	Events []Event       `json:"events"`
}

func notifyCourier(baseURL string, ch models.Channel, payload *receivePayload) error {
	url := fmt.Sprintf("%s/c/twc/%s/receive", baseURL, ch.UUID())
	body := jsonx.MustMarshal(payload)
	request, _ := httpx.NewRequest("POST", url, bytes.NewReader(body), nil)

	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		return errors.Wrap(err, "error connecting courier")
	} else if resp.StatusCode/100 != 2 {
		return errors.New("courier returned non-2XX status")
	}

	slog.Debug("courier notified", "event", body, "status", resp.StatusCode)
	return nil
}

func NotifyChatStarted(cfg *runtime.Config, ch models.Channel, contact *models.Contact) error {
	return notifyCourier(cfg.Courier, ch, &receivePayload{
		ChatID: contact.ChatID,
		Secret: ch.Secret(),
		Events: []Event{newChatStartedEvent()},
	})
}

func NotifyMsgIn(cfg *runtime.Config, ch models.Channel, contact *models.Contact, e *events.MsgIn) error {
	return notifyCourier(cfg.Courier, ch, &receivePayload{
		ChatID: contact.ChatID,
		Secret: ch.Secret(),
		Events: []Event{newMsgInEvent(e.Text)},
	})
}
