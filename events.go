package tembachat

type Event interface {
	Type() string
}

type baseEvent struct {
	Type_ string `json:"type"`
}

func (e *baseEvent) Type() string {
	return e.Type_
}

type chatStartedEvent struct {
	baseEvent
	Identifier string `json:"identifier"`
}

func newChatStartedEvent(identifier string) *chatStartedEvent {
	return &chatStartedEvent{baseEvent: baseEvent{Type_: "chat_started"}, Identifier: identifier}
}

type msgOutEvent struct {
	baseEvent
	Text string `json:"text"`
}

func newMsgOutEvent(text string) *msgOutEvent {
	return &msgOutEvent{baseEvent: baseEvent{Type_: "msg_out"}, Text: text}
}

type msgInEvent struct {
	baseEvent
	Text string `json:"text"`
}
