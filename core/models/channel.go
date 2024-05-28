package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/gocommon/uuids"
)

type ChannelID int64
type ChannelUUID uuids.UUID

type Channel struct {
	ID     ChannelID      `json:"id"`
	UUID   ChannelUUID    `json:"uuid"`
	OrgID  OrgID          `json:"org_id"`
	Config map[string]any `json:"config"`
}

func (c *Channel) Secret() string {
	s, _ := c.Config["secret"].(string)
	return s
}

const sqlSelectChannel = `
SELECT row_to_json(r) FROM (
	SELECT id, uuid, org_id, config 
	FROM channels_channel 
	WHERE uuid = $1 AND channel_type = 'CHP' AND is_active
) r`

func LoadChannel(ctx context.Context, rt *runtime.Runtime, uuid ChannelUUID) (*Channel, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectChannel, uuid)
	if err != nil {
		return nil, fmt.Errorf("error querying channel: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	ch := &Channel{}
	if err := dbutil.ScanJSON(rows, ch); err != nil {
		return nil, fmt.Errorf("error scanning channel: %w", err)
	}
	return ch, nil
}
