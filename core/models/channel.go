package models

import (
	"context"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type OrgID int
type ChannelID int64
type ChannelUUID uuids.UUID

type Channel interface {
	UUID() ChannelUUID
	OrgID() OrgID
	Secret() string
}

type channel struct {
	UUID_   ChannelUUID    `json:"uuid"`
	OrgID_  OrgID          `json:"org_id"`
	Config_ map[string]any `json:"config"`
}

func (c *channel) UUID() ChannelUUID { return c.UUID_ }
func (c *channel) OrgID() OrgID      { return c.OrgID_ }
func (c *channel) Secret() string {
	s, _ := c.Config_["secret"].(string)
	return s
}

const sqlSelectChannel = `
SELECT row_to_json(r) FROM (
	SELECT uuid, org_id, config FROM channels_channel WHERE uuid = $1 AND channel_type = 'TWC' AND is_active
) r`

func LoadChannel(ctx context.Context, rt *runtime.Runtime, uuid ChannelUUID) (Channel, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectChannel, uuid)
	if err != nil {
		return nil, errors.Wrap(err, "error querying channel")
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("channel query returned no rows")
	}
	ch := &channel{}
	if err := dbutil.ScanJSON(rows, ch); err != nil {
		return nil, errors.Wrap(err, "error scanning channel")
	}
	return ch, nil
}
