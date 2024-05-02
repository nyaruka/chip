package courier

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type payload struct {
	ChatID models.ChatID `json:"chat_id"`
	Secret string        `json:"secret"`
	Events []Event       `json:"events"`
}

func notifyCourier(baseURL string, ch *models.Channel, payload *payload) error {
	url := fmt.Sprintf("%s/c/twc/%s/receive", baseURL, ch.UUID)
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

func StartChat(cfg *runtime.Config, ch *models.Channel, chatID models.ChatID) error {
	return notifyCourier(cfg.Courier, ch, &payload{
		ChatID: chatID,
		Secret: ch.Secret(),
		Events: []Event{newChatStartedEvent()},
	})
}

func CreateMsg(cfg *runtime.Config, ch *models.Channel, contact *models.Contact, text string) error {
	return notifyCourier(cfg.Courier, ch, &payload{
		ChatID: contact.ChatID,
		Secret: ch.Secret(),
		Events: []Event{newMsgInEvent(text)},
	})
}
