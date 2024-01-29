package models

import (
	"context"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type ContactID int64
type URNID int64
type ChatID string

var chatIDRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func NewChatID() ChatID {
	return ChatID(random.String(24, chatIDRunes))
}

type Contact interface {
	OrgID() OrgID
	ChatID() ChatID
	Email() string
}

type contact struct {
	OrgID_  OrgID  `json:"org_id"`
	ChatID_ ChatID `json:"chat_id"`
	Email_  string `json:"email"`
}

func (c *contact) OrgID() OrgID   { return c.OrgID_ }
func (c *contact) ChatID() ChatID { return c.ChatID_ }
func (c *contact) Email() string  { return c.Email_ }

func NewContact(ch Channel) Contact {
	return &contact{OrgID_: ch.OrgID(), ChatID_: NewChatID()}
}

const sqlSelectURN = `
SELECT row_to_json(r) FROM (
	SELECT org_id, path AS chat_id, display AS email FROM contacts_contacturn WHERE org_id = $1 AND identity = $2
) r`

func LoadContact(ctx context.Context, rt *runtime.Runtime, channel Channel, chatID ChatID) (Contact, error) {
	// convert chatID to a webchat URN amd check that's valid
	urn, err := urns.NewURNFromParts(urns.WebChatScheme, string(chatID), "", "")
	if err != nil {
		return nil, err
	}

	rows, err := rt.DB.QueryContext(ctx, sqlSelectURN, channel.OrgID(), urn.Identity())
	if err != nil {
		return nil, errors.Wrap(err, "error querying contact")
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("contact query returned no rows")
	}
	c := &contact{}
	if err := dbutil.ScanJSON(rows, c); err != nil {
		return nil, errors.Wrap(err, "error scanning contact")
	}
	return c, nil
}
