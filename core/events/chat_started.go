package events

func init() {
	registerType(TypeChatStarted, func() Event { return &ChatStarted{} })
}

const TypeChatStarted string = "chat_started"

type ChatStarted struct {
	baseEvent
	ChatID string `json:"chat_id"`
}

func NewChatStarted(chatID string) *ChatStarted {
	return &ChatStarted{baseEvent: baseEvent{Type_: TypeChatStarted}, ChatID: chatID}
}
