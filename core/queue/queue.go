package queue

import (
	"github.com/nyaruka/tembachat/core/models"
)

type Outboxes struct {
	KeyBase string
}

func (q *Outboxes) AddMessage(chatID models.ChatID, m *models.MsgOut) error {
	// TODO
	return nil
}
