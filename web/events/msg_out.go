package events

import (
	"time"

	"github.com/nyaruka/tembachat/core/models"
)

const TypeMsgOut string = "msg_out"

type MsgOutEvent struct {
	baseEvent

	MsgID  models.MsgID     `json:"msg_id"`
	Text   string           `json:"text"`
	Origin models.MsgOrigin `json:"origin"`
	User   *User            `json:"user,omitempty"`
}

func NewMsgOut(t time.Time, id models.MsgID, text string, origin models.MsgOrigin, user *User) *MsgOutEvent {
	return &MsgOutEvent{
		baseEvent: baseEvent{Type_: TypeMsgOut, Time_: t},
		MsgID:     id,
		Text:      text,
		Origin:    origin,
		User:      user,
	}
}
