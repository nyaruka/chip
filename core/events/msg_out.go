package events

import "github.com/nyaruka/tembachat/core/models"

func init() {
	registerType(TypeMsgOut, func() Event { return &MsgOut{} })
}

const TypeMsgOut string = "msg_out"

type MsgOut struct {
	baseEvent
	Text   string           `json:"text"`
	Origin models.MsgOrigin `json:"origin"`
	User   models.User      `json:"user,omitempty"`
}

func NewMsgOut(text string, origin models.MsgOrigin, user models.User) *MsgOut {
	return &MsgOut{baseEvent: baseEvent{Type_: TypeMsgOut}, Text: text, Origin: origin, User: user}
}
