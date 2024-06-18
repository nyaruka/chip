package events

import "github.com/nyaruka/chip/core/models"

const TypeChatOut string = "chat_out"

type ChatOutEvent struct {
	baseEvent

	MsgOut *models.MsgOut `json:"msg_out,omitempty"`
}

func NewChatMsgOut(MsgOut *models.MsgOut) *ChatOutEvent {
	return &ChatOutEvent{
		baseEvent: baseEvent{Type_: TypeChatOut},
		MsgOut:    MsgOut,
	}
}
