package events

import "github.com/nyaruka/tembachat/core/models"

const TypeChatResumed string = "chat_resumed"

type ChatResumed struct {
	baseEvent

	ChatID models.ChatID `json:"chat_id"`
	Email  string        `json:"email"`
}

func NewChatResumed(chatID models.ChatID, email string) *ChatResumed {
	return &ChatResumed{baseEvent: baseEvent{Type_: TypeChatResumed}, ChatID: chatID, Email: email}
}
