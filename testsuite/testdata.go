package testsuite

import (
	"github.com/lib/pq"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

func InsertOrg(rt *runtime.Runtime, name string) int {
	row := rt.DB.QueryRow(`INSERT INTO orgs_org(name, is_active) VALUES($1, TRUE) RETURNING id`, name)
	var id int
	must(row.Scan(&id))
	return id
}

func InsertChannel(rt *runtime.Runtime, orgID int, channelType, name, address string, schemes []string) webchat.ChannelUUID {
	uuid := webchat.ChannelUUID(uuids.New())
	_, err := rt.DB.Exec(
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, address, schemes, role, config, log_policy, is_active, created_on, modified_on, created_by_id, modified_by_id) 
		VALUES($1, $2, $3, $4, $5, $6, 'SR', '{}', 'A', TRUE, NOW(), NOW(), 1, 1)`, uuid, orgID, channelType, name, address, pq.Array(schemes),
	)
	noError(err)
	return uuid
}
