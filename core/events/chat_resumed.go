package events

func init() {
	registerType(TypeChatResumed, func() Event { return &ChatResumed{} })
}

const TypeChatResumed string = "chat_resumed"

type ChatResumed struct {
	baseEvent
	ChatID string `json:"chat_id"`
	Email  string `json:"email"`
}

func NewChatResumed(chatID, email string) *ChatResumed {
	return &ChatResumed{baseEvent: baseEvent{Type_: TypeChatResumed}, ChatID: chatID, Email: email}
}
