package events

import (
	"time"

	"github.com/nyaruka/tembachat/core/models"
)

const TypeHistory string = "history"

const TypeMsgIn string = "msg_in"

type MsgInEvent struct {
	baseEvent

	MsgID models.MsgID `json:"msg_id"`
	Text  string       `json:"text"`
}

func NewMsgIn(t time.Time, id models.MsgID, text string) Event {
	return &MsgInEvent{
		baseEvent: baseEvent{Type_: TypeMsgIn, Time_: t},
		MsgID:     id,
		Text:      text,
	}
}

type HistoryEvent struct {
	baseEvent

	History []Event `json:"history"`
}

func NewHistory(t time.Time, history []Event) *HistoryEvent {
	return &HistoryEvent{baseEvent: baseEvent{Type_: TypeHistory, Time_: t}, History: history}
}
