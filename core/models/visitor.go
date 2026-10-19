package models

import (
	"context"

	"github.com/nyaruka/gocommon/urns"
)

// Visitor is who a chat ID identifies: the contact courier created when the chat started, and the webchat URN
// which carries the chat ID as its path
type Visitor struct {
	ContactID ContactID `db:"contact_id"`
	URNID     URNID     `db:"urn_id"`
}

// ChatURN returns the webchat URN for the given chat ID, or an error if the chat ID isn't the right shape to be
// one - which is how a chat ID is validated
func ChatURN(chatID string) (urns.URN, error) {
	return urns.NewFromParts(urns.WebChat.Prefix, chatID, nil, "")
}

const sqlSelectVisitor = `
SELECT u.contact_id, u.id AS urn_id
  FROM contacts_contacturn u
  JOIN contacts_contact c ON c.id = u.contact_id
 WHERE u.org_id = $1 AND u.identity = $2 AND u.channel_id = $3 AND c.is_active`

// LoadVisitor loads the visitor chatting on the given channel under the given chat ID, or nil if there's no such
// chat: chat IDs are only ever minted when a chat starts, so one we don't know isn't a new visitor but a bad
// request. The URN has to belong to the channel - a URN always shares its channel's workspace, so this also keeps a
// chat ID from one workspace's channel from reaching another's - and to an active contact, so releasing a contact
// closes their chat immediately.
func LoadVisitor(ctx context.Context, db DBorTx, ch *Channel, chatID string) (*Visitor, error) {
	urn, err := ChatURN(chatID)
	if err != nil {
		return nil, nil
	}
	return getOne[Visitor](ctx, db, sqlSelectVisitor, ch.OrgID(), urn.Identity(), ch.ID())
}
