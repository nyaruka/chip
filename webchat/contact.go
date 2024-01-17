package webchat

import (
	"context"

	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type ContactID int64
type URNID int64

const sqlSelectURN = `SELECT id FROM contacts_contacturn WHERE org_id = $1 AND identity = $2`

func URNExists(ctx context.Context, rt *runtime.Runtime, ch Channel, urn urns.URN) (bool, error) {
	rows, err := rt.DB.Query(sqlSelectURN, ch.OrgID(), urn.Identity())
	if err != nil {
		return false, errors.Wrap(err, "error querying URN")
	}
	return rows.Next(), nil
}
