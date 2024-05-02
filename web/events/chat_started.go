package events

import (
	"time"

	"github.com/nyaruka/tembachat/core/models"
)

const TypeChatStarted string = "chat_started"

type ChatStarted struct {
	baseEvent

	ChatID models.ChatID `json:"chat_id"`
}

func NewChatStarted(t time.Time, chatID models.ChatID) *ChatStarted {
	return &ChatStarted{baseEvent: baseEvent{Type_: TypeChatStarted, Time_: t}, ChatID: chatID}
}
