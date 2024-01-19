package webchat

type Event interface {
	Type() string
}

type baseEvent struct {
	Type_ string `json:"type"`
}

func (e *baseEvent) Type() string {
	return e.Type_
}

type ChatStartedEvent struct {
	baseEvent
	Identifier string `json:"identifier"`
}

func NewChatStartedEvent(identifier string) *ChatStartedEvent {
	return &ChatStartedEvent{baseEvent: baseEvent{Type_: "chat_started"}, Identifier: identifier}
}

type ChatResumedEvent struct {
	baseEvent
	Identifier string `json:"identifier"`
}

func NewChatResumedEvent(identifier string) *ChatResumedEvent {
	return &ChatResumedEvent{baseEvent: baseEvent{Type_: "chat_resumed"}, Identifier: identifier}
}

type MsgOutEvent struct {
	baseEvent
	Text   string `json:"text"`
	Origin string `json:"origin"`
	User   User   `json:"user,omitempty"`
}

func NewMsgOutEvent(text, origin string, user User) *MsgOutEvent {
	return &MsgOutEvent{baseEvent: baseEvent{Type_: "msg_out"}, Text: text, Origin: origin, User: user}
}

type MsgInEvent struct {
	baseEvent
	Text string `json:"text"`
}
