package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/urns"
)

type ContactID int64
type URNID int64
type ChatID string

var chatIDRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func NewChatID() ChatID {
	return ChatID(random.String(24, chatIDRunes))
}

type Contact struct {
	ID     ContactID `json:"id"`
	OrgID  OrgID     `json:"org_id"`
	URNID  URNID     `json:"urn_id"`
	ChatID ChatID    `json:"chat_id"`
	Email  string    `json:"email"`
}

func (c *Contact) UpdateEmail(ctx context.Context, rt *runtime.Runtime, email string) error {
	c.Email = email

	urn, _ := urns.New(urns.WebChat, string(c.ChatID))

	row := rt.DB.QueryRowContext(ctx,
		`UPDATE contacts_contacturn SET display = $3 WHERE org_id = $1 AND identity = $2 RETURNING contact_id`, c.OrgID, urn, email,
	)

	var contactID ContactID
	if err := row.Scan(&contactID); err != nil {
		return fmt.Errorf("error updating URN display: %w", err)
	}

	_, err := rt.DB.ExecContext(ctx, `UPDATE contacts_contact SET modified_on = NOW() WHERE id = $1`, contactID)
	if err != nil {
		return fmt.Errorf("error updating contact modified_on: %w", err)
	}

	return nil
}

const sqlSelectContact = `
SELECT row_to_json(r) FROM (
	SELECT contact_id AS id, org_id, id AS urn_id, path AS chat_id, display AS email 
	FROM contacts_contacturn 
	WHERE org_id = $1 AND identity = $2
) r`

func LoadContact(ctx context.Context, rt *runtime.Runtime, orgID OrgID, chatID ChatID) (*Contact, error) {
	// convert chatID to a webchat URN amd check that's valid
	urn, err := urns.New(urns.WebChat, string(chatID))
	if err != nil {
		return nil, err
	}

	rows, err := rt.DB.QueryContext(ctx, sqlSelectContact, orgID, urn.Identity())
	if err != nil {
		return nil, fmt.Errorf("error querying contact: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	c := &Contact{}
	if err := dbutil.ScanJSON(rows, c); err != nil {
		return nil, fmt.Errorf("error scanning contact: %w", err)
	}
	return c, nil
}
