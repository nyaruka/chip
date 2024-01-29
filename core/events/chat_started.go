package events

import "github.com/nyaruka/tembachat/core/models"

func init() {
	registerType(TypeChatStarted, func() Event { return &ChatStarted{} })
}

const TypeChatStarted string = "chat_started"

type ChatStarted struct {
	baseEvent
	ChatID models.ChatID `json:"chat_id"`
}

func NewChatStarted(chatID models.ChatID) *ChatStarted {
	return &ChatStarted{baseEvent: baseEvent{Type_: TypeChatStarted}, ChatID: chatID}
}
