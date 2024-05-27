package models

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
)

type MsgID int64
type MsgOrigin string
type MsgStatus string
type MsgDirection string

const (
	NilMsgID MsgID = 0

	MsgOriginFlow      MsgOrigin = "flow"
	MsgOriginBroadcast MsgOrigin = "broadcast"
	MsgOriginTicket    MsgOrigin = "ticket"
	MsgOriginChat      MsgOrigin = "chat"

	MsgStatusSent MsgStatus = "sent"

	DirectionIn  MsgDirection = "I"
	DirectionOut MsgDirection = "O"
)

type MsgOut struct {
	ID          MsgID     `json:"id"`
	ChatID      ChatID    `json:"chat_id"`
	Text        string    `json:"text"`
	Attachments []string  `json:"attachments,omitempty"`
	Origin      MsgOrigin `json:"origin"`
	UserID      UserID    `json:"user_id"`
	Time        time.Time `json:"time"`
}

func NewMsgOut(id MsgID, chatID ChatID, text string, attachments []string, origin MsgOrigin, u *User, t time.Time) *MsgOut {
	var userID UserID
	if u != nil {
		userID = u.ID
	}

	return &MsgOut{ID: id, ChatID: chatID, Text: text, Origin: origin, UserID: userID, Time: t}
}

type Msg struct {
	ID          MsgID        `json:"id"`
	Text        string       `json:"text"`
	Attachments []string     `json:"attachments"`
	Direction   MsgDirection `json:"direction"`
	BroadcastID BroadcastID  `json:"broadcast_id"`
	FlowID      FlowID       `json:"flow_id"`
	TicketID    TicketID     `json:"ticket_id"`
	CreatedByID UserID       `json:"created_by_id"`
	CreatedOn   time.Time    `json:"created_on"`
}

func (m *Msg) Origin() MsgOrigin {
	if m.FlowID != NilFlowID {
		return MsgOriginFlow
	} else if m.BroadcastID != NilBroadcastID {
		return MsgOriginBroadcast
	} else if m.TicketID != NilTicketID {
		return MsgOriginTicket
	}
	return MsgOriginChat
}

const sqlSelectContactMessages = `
SELECT row_to_json(r) FROM (
    SELECT id, text, attachments, direction, broadcast_id, flow_id, ticket_id, created_by_id, created_on
      FROM msgs_msg 
     WHERE contact_id = $1 AND msg_type = 'T' AND visibility IN ('V', 'A') AND created_on < $2
  ORDER BY created_on DESC, id DESC 
     LIMIT $3
) r`

func LoadContactMessages(ctx context.Context, rt *runtime.Runtime, contactID ContactID, before time.Time, limit int) ([]*Msg, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectContactMessages, contactID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying contact messages: %w", err)
	}
	defer rows.Close()

	msgs := make([]*Msg, 0)

	for rows.Next() {
		msg := &Msg{}
		if err := dbutil.ScanJSON(rows, msg); err != nil {
			return nil, fmt.Errorf("error scanning msg row: %w", err)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}
