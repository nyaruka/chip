package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// MsgDirection is the direction of a message
type MsgDirection string

const (
	MsgIncoming MsgDirection = "I"
	MsgOutgoing MsgDirection = "O"
)

const (
	QuickReplyTypeText = "text"
)

// QuickReply is a quick reply as the platform stores them on a message - the same shape the engine's msg events
// carry, so the widget can render them the same way
type QuickReply struct {
	Type  string `json:"type"            validate:"required"`
	Text  string `json:"text,omitempty"`
	Extra string `json:"extra,omitempty"`
}

// QuickReplies is a slice of quick replies that can be scanned from a nullable JSONB column
type QuickReplies []QuickReply

func (qr *QuickReplies) Scan(value any) error {
	if value == nil {
		*qr = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan quick replies: expected []byte, got %T", value)
	}
	return json.Unmarshal(b, qr)
}

// TextQuickReplies returns just the text quick replies from the given quick replies, which are the only kind the
// widget renders - so like any other channel that doesn't support a kind, the rest are dropped
func TextQuickReplies(qrs []QuickReply) []QuickReply {
	var text []QuickReply
	for _, qr := range qrs {
		if qr.Type == QuickReplyTypeText {
			text = append(text, qr)
		}
	}
	return text
}

// ChatMsg is a message in a conversation as the history endpoint shows it, in either direction
type ChatMsg struct {
	UUID         MsgUUID        `db:"uuid"`
	Direction    MsgDirection   `db:"direction"`
	Text         string         `db:"text"`
	Attachments  pq.StringArray `db:"attachments"`
	QuickReplies QuickReplies   `db:"quickreplies"`
	CreatedOn    time.Time      `db:"created_on"`

	// the user who sent an outgoing message, if it was sent by one rather than by a flow - joined in nullable
	// columns since most messages have no sender, see Sender
	UserID_        sql.NullInt64  `db:"user_id"`
	UserUUID_      sql.NullString `db:"user_uuid"`
	UserFirstName_ sql.NullString `db:"user_first_name"`
	UserLastName_  sql.NullString `db:"user_last_name"`
	UserAvatar_    sql.NullString `db:"user_avatar"`
}

// Sender returns the user who sent the message, or nil if it wasn't sent by a user
func (m *ChatMsg) Sender() *User {
	if !m.UserID_.Valid {
		return nil
	}
	return &User{
		ID_:        UserID(m.UserID_.Int64),
		UUID_:      UserUUID(m.UserUUID_.String),
		FirstName_: m.UserFirstName_.String,
		LastName_:  m.UserLastName_.String,
		Avatar_:    m.UserAvatar_,
	}
}

// filtering by URN rather than contact both scopes the query to a single conversation - a contact can hold more
// than one chat URN - and is what makes it cheap, as the URN foreign key is indexed. Message UUIDs are v7 so
// UUID order is message order, which is what makes them the paging key.
const sqlSelectChatMsgs = `
SELECT m.uuid, m.direction, m.text, m.attachments, m.quickreplies, m.created_on,
       u.id AS user_id, u.uuid AS user_uuid, u.first_name AS user_first_name, u.last_name AS user_last_name, u.avatar AS user_avatar
  FROM msgs_msg m
  LEFT JOIN users_user u ON u.id = m.created_by_id
 WHERE m.contact_urn_id = $1 AND m.channel_id = $2 AND m.visibility = 'V' AND ($3::uuid IS NULL OR m.uuid < $3)
 ORDER BY m.uuid DESC
 LIMIT $4`

// GetChatMsgs returns the visible messages in both directions of the given visitor's conversation on the given
// channel, newest first, optionally only those before the given message UUID.
func GetChatMsgs(ctx context.Context, db DBorTx, ch *Channel, visitor *Visitor, before MsgUUID, limit int) ([]*ChatMsg, error) {
	var beforeUUID *string
	if before != "" {
		beforeUUID = (*string)(&before)
	}

	msgs := make([]*ChatMsg, 0, limit)
	if err := db.SelectContext(ctx, &msgs, sqlSelectChatMsgs, visitor.URNID, ch.ID(), beforeUUID, limit); err != nil {
		return nil, fmt.Errorf("error loading chat messages: %w", err)
	}
	return msgs, nil
}
