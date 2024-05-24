package courier

import "github.com/nyaruka/chip/core/models"

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
}

func newChatStartedEvent() Event {
	return &chatStartedEvent{
		baseEvent: baseEvent{Type_: "chat_started"},
	}
}

type msgIn struct {
	Text        string   `json:"text"`
	Attachments []string `json:"attachments"`
}

type msgInEvent struct {
	baseEvent
	Msg msgIn `json:"msg"`
}

func newMsgInEvent(text string, attachments []string) Event {
	return &msgInEvent{
		baseEvent: baseEvent{Type_: "msg_in"},
		Msg:       msgIn{Text: text, Attachments: attachments},
	}
}

type msgStatus struct {
	MsgID  models.MsgID     `json:"msg_id"`
	Status models.MsgStatus `json:"status"`
}

type msgStatusEvent struct {
	baseEvent
	Status msgStatus `json:"status"`
}

func newMsgStatusEvent(msgID models.MsgID, status models.MsgStatus) Event {
	return &msgStatusEvent{
		baseEvent: baseEvent{Type_: "msg_status"},
		Status:    msgStatus{MsgID: msgID, Status: status},
	}
}
