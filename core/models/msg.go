package models

import (
	"context"
	"fmt"
	"time"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
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

type Msg struct {
	ID        MsgID        `json:"id"`
	Text      string       `json:"text"`
	Direction MsgDirection `json:"direction"`
	CreatedBy UserID       `json:"created_by_id"`
	CreatedOn time.Time    `json:"created_on"`
}

const sqlSelectContactMessages = `
SELECT row_to_json(r) FROM (
    SELECT id, text, direction, created_by_id, created_on 
      FROM msgs_msg 
     WHERE contact_id = $1 AND msg_type = 'T' AND visibility IN ('V', 'A') %s
  ORDER BY created_on DESC, id DESC 
     LIMIT %d
) r`

func LoadContactMessages(ctx context.Context, rt *runtime.Runtime, contactID ContactID, beforeCreatedOn *time.Time, limit int) ([]*Msg, error) {
	var q string
	var params []any

	if beforeCreatedOn != nil {
		q = fmt.Sprintf(sqlSelectContactMessages, "AND created_on < $2", limit)
		params = []any{contactID, *beforeCreatedOn}
	} else {
		q = fmt.Sprintf(sqlSelectContactMessages, "", limit)
		params = []any{contactID}
	}

	rows, err := rt.DB.QueryContext(ctx, q, params...)
	if err != nil {
		return nil, errors.Wrap(err, "error querying contact messages")
	}
	defer rows.Close()

	msgs := make([]*Msg, 0)

	for rows.Next() {
		msg := &Msg{}
		if err := dbutil.ScanJSON(rows, msg); err != nil {
			return nil, errors.Wrap(err, "error scanning msg row")
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}
