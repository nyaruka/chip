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
	Text string `json:"text"`
}

type msgInEvent struct {
	baseEvent
	Msg msgIn `json:"msg"`
}

func newMsgInEvent(text string) Event {
	return &msgInEvent{
		baseEvent: baseEvent{Type_: "msg_in"},
		Msg:       msgIn{Text: text},
	}
}

type MsgStatus string

const (
	MsgStatusDelivered MsgStatus = "delivered"
)

type msgStatusUpdate struct {
	MsgID  models.MsgID `json:"msg_id"`
	Status MsgStatus    `json:"status"`
}

type msgStatusEvent struct {
	baseEvent
	Status msgStatusUpdate `json:"status"`
}

func newMsgStatusEvent(msgID models.MsgID, status MsgStatus) Event {
	return &msgStatusEvent{
		baseEvent: baseEvent{Type_: "msg_status"},
		Status:    msgStatusUpdate{MsgID: msgID, Status: status},
	}
}
