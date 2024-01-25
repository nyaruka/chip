package events

import "github.com/nyaruka/tembachat/core/models"

type Event interface {
	Type() string
}

type baseEvent struct {
	Type_ string `json:"type"`
}

func (e *baseEvent) Type() string {
	return e.Type_
}

type ChatStarted struct {
	baseEvent
	Identifier string `json:"identifier"`
}

func NewChatStarted(identifier string) *ChatStarted {
	return &ChatStarted{baseEvent: baseEvent{Type_: "chat_started"}, Identifier: identifier}
}

type ChatResumed struct {
	baseEvent
	Identifier string `json:"identifier"`
}

func NewChatResumed(identifier string) *ChatResumed {
	return &ChatResumed{baseEvent: baseEvent{Type_: "chat_resumed"}, Identifier: identifier}
}

type MsgOut struct {
	baseEvent
	Text   string      `json:"text"`
	Origin string      `json:"origin"`
	User   models.User `json:"user,omitempty"`
}

func NewMsgOut(text, origin string, user models.User) *MsgOut {
	return &MsgOut{baseEvent: baseEvent{Type_: "msg_out"}, Text: text, Origin: origin, User: user}
}

type MsgIn struct {
	baseEvent
	Text string `json:"text"`
}
