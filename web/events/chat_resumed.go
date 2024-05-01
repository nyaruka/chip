package events

import (
	"time"

	"github.com/nyaruka/tembachat/core/models"
)

const TypeChatResumed string = "chat_resumed"

type ChatResumed struct {
	baseEvent

	ChatID models.ChatID `json:"chat_id"`
	Email  string        `json:"email"`
}

func NewChatResumed(t time.Time, chatID models.ChatID, email string) *ChatResumed {
	return &ChatResumed{baseEvent: baseEvent{Type_: TypeChatResumed, Time_: t}, ChatID: chatID, Email: email}
}
