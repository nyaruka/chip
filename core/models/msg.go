package models

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/gocommon/uuids"
)

type MsgID int64
type MsgUUID uuids.UUID
type MsgOrigin string
type MsgDirection string

const (
	NilMsgID MsgID = 0

	MsgOriginFlow      MsgOrigin = "flow"
	MsgOriginBroadcast MsgOrigin = "broadcast"
	MsgOriginTicket    MsgOrigin = "ticket"
	MsgOriginChat      MsgOrigin = "chat"

	DirectionIn  MsgDirection = "I"
	DirectionOut MsgDirection = "O"
)

type MsgIn struct {
	UUID MsgUUID   `json:"uuid"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

func NewMsgIn(uuid MsgUUID, text string, t time.Time) *MsgIn {
	return &MsgIn{UUID: uuid, Text: text, Time: t}
}

type MsgOut struct {
	UUID        MsgUUID   `json:"uuid"`
	Text        string    `json:"text"`
	Attachments []string  `json:"attachments,omitempty"`
	Origin      MsgOrigin `json:"origin"`
	User        *User     `json:"user,omitempty"`
	Time        time.Time `json:"time"`
}

func NewMsgOut(uuid MsgUUID, text string, attachments []string, origin MsgOrigin, user *User, t time.Time) *MsgOut {
	return &MsgOut{UUID: uuid, Text: text, Attachments: attachments, Origin: origin, User: user, Time: t}
}

type DBMsg struct {
	UUID        MsgUUID      `json:"uuid"`
	Text        string       `json:"text"`
	Attachments []string     `json:"attachments"`
	Direction   MsgDirection `json:"direction"`
	BroadcastID BroadcastID  `json:"broadcast_id"`
	FlowID      FlowID       `json:"flow_id"`
	TicketID    TicketID     `json:"ticket_id"`
	CreatedByID UserID       `json:"created_by_id"`
	CreatedOn   time.Time    `json:"created_on"`
}

func (m *DBMsg) ToMsgIn() *MsgIn {
	if m.Direction != DirectionIn {
		panic("can only be called on an inbound message")
	}

	return NewMsgIn(m.UUID, m.Text, m.CreatedOn)
}

func (m *DBMsg) ToMsgOut(ctx context.Context, store Store) (*MsgOut, error) {
	if m.Direction != DirectionOut {
		panic("can only be called on an outbound message")
	}

	var user *User
	var err error
	if m.CreatedByID != NilUserID {
		user, err = store.GetUser(ctx, m.CreatedByID)
		if err != nil {
			return nil, fmt.Errorf("error fetching user: %w", err)
		}
	}

	return NewMsgOut(m.UUID, m.Text, m.Attachments, m.origin(), user, m.CreatedOn), nil
}

func (m *DBMsg) origin() MsgOrigin {
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
    SELECT uuid, text, attachments, direction, broadcast_id, flow_id, ticket_id, created_by_id, created_on
      FROM msgs_msg 
     WHERE contact_id = $1 AND msg_type = 'T' AND visibility IN ('V', 'A') AND created_on < $2
  ORDER BY created_on DESC, id DESC 
     LIMIT $3
) r`

func LoadContactMessages(ctx context.Context, rt *runtime.Runtime, contactID ContactID, before time.Time, limit int) ([]*DBMsg, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectContactMessages, contactID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying contact messages: %w", err)
	}
	defer rows.Close()

	msgs := make([]*DBMsg, 0)

	for rows.Next() {
		msg := &DBMsg{}
		if err := dbutil.ScanJSON(rows, msg); err != nil {
			return nil, fmt.Errorf("error scanning msg row: %w", err)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}
