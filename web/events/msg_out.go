package events

import (
	"time"

	"github.com/nyaruka/tembachat/core/models"
)

const TypeMsgOut string = "msg_out"

type MsgOutEvent struct {
	baseEvent

	MsgID       models.MsgID     `json:"msg_id"`
	Text        string           `json:"text"`
	Attachments []string         `json:"attachments,omitempty"`
	Origin      models.MsgOrigin `json:"origin"`
	User        *User            `json:"user,omitempty"`
}

func NewMsgOut(t time.Time, id models.MsgID, text string, attachments []string, origin models.MsgOrigin, user *User) *MsgOutEvent {
	return &MsgOutEvent{
		baseEvent:   baseEvent{Type_: TypeMsgOut, Time_: t},
		MsgID:       id,
		Text:        text,
		Attachments: attachments,
		Origin:      origin,
		User:        user,
	}
}
