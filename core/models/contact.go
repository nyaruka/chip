package models

import (
	"context"
	"strings"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type ContactID int64

type URNID int64

const sqlSelectURN = `SELECT id FROM contacts_contacturn WHERE org_id = $1 AND identity = $2`

func URNExists(ctx context.Context, rt *runtime.Runtime, ch Channel, urn urns.URN) (bool, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectURN, ch.OrgID(), urn.Identity())
	if err != nil {
		return false, errors.Wrap(err, "error querying URN")
	}
	defer rows.Close()

	return rows.Next(), nil
}

func NewURN(chatID, email string) urns.URN {
	path := chatID
	if email != "" {
		path += ":" + email
	}
	return urns.URN(urns.WebChatScheme + ":" + path)
}

func ParseURN(u urns.URN) (string, string) {
	parts := strings.SplitN(u.Path(), ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}
