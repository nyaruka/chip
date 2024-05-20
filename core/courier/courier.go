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

// Courier is the interface for interacting with a courier instance or a mock
type Courier interface {
	StartChat(ch *models.Channel, chatID models.ChatID) error
	CreateMsg(ch *models.Channel, contact *models.Contact, text string, attachments []string) error
}

type courier struct {
	cfg *runtime.Config
}

// NewCourier creates a new courier instance using the provided configuration
func NewCourier(cfg *runtime.Config) Courier {
	return &courier{cfg: cfg}
}

type payload struct {
	ChatID models.ChatID `json:"chat_id"`
	Secret string        `json:"secret"`
	Events []Event       `json:"events"`
}

func (c *courier) request(ch *models.Channel, payload *payload) error {
	url := fmt.Sprintf("%s/c/twc/%s/receive", c.cfg.Courier, ch.UUID)
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

func (c *courier) StartChat(ch *models.Channel, chatID models.ChatID) error {
	return c.request(ch, &payload{
		ChatID: chatID,
		Secret: ch.Secret(),
		Events: []Event{newChatStartedEvent()},
	})
}

func (c *courier) CreateMsg(ch *models.Channel, contact *models.Contact, text string, attachments []string) error {
	return c.request(ch, &payload{
		ChatID: contact.ChatID,
		Secret: ch.Secret(),
		Events: []Event{newMsgInEvent(text, attachments)},
	})
}
