package models

import (
	"time"
)

type MsgID int64

type MsgOrigin string

const (
	MsgOriginFlow      MsgOrigin = "flow"
	MsgOriginBroadcast MsgOrigin = "broadcast"
	MsgOriginTicket    MsgOrigin = "ticket"
	MsgOriginChat      MsgOrigin = "chat"
)

type MsgOut struct {
	ID     MsgID     `json:"id"`
	ChatID ChatID    `json:"chat_id"`
	Text   string    `json:"text"`
	Origin MsgOrigin `json:"origin"`
	UserID UserID    `json:"user_id"`
	Time   time.Time `json:"time"`
}

func NewMsgOut(id MsgID, chatID ChatID, text string, origin MsgOrigin, u User, t time.Time) *MsgOut {
	var userID UserID
	if u != nil {
		userID = u.ID()
	}

	return &MsgOut{ID: id, ChatID: chatID, Text: text, Origin: origin, UserID: userID, Time: t}
}
