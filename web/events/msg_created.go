package events

import "github.com/nyaruka/tembachat/core/models"

const TypeMsgCreated string = "msg_created"

type MsgCreated struct {
	baseEvent

	Text   string           `json:"text"`
	Origin models.MsgOrigin `json:"origin"`
	User   models.User      `json:"user,omitempty"`
}

func NewMsgCreated(text string, origin models.MsgOrigin, user models.User) *MsgCreated {
	return &MsgCreated{baseEvent: baseEvent{Type_: TypeMsgCreated}, Text: text, Origin: origin, User: user}
}
