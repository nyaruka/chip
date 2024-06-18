package events

import "github.com/nyaruka/chip/core/models"

const TypeHistory string = "history"

type HistoryItem struct {
	MsgIn  *models.MsgIn  `json:"msg_in,omitempty"`
	MsgOut *models.MsgOut `json:"msg_out,omitempty"`
}

type HistoryEvent struct {
	baseEvent

	History []*HistoryItem `json:"history"`
}

func NewHistory(history []*HistoryItem) *HistoryEvent {
	return &HistoryEvent{baseEvent: baseEvent{Type_: TypeHistory}, History: history}
}
