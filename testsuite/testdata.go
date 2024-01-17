package testsuite

import (
	"github.com/lib/pq"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

func InsertOrg(rt *runtime.Runtime, name string) webchat.OrgID {
	row := rt.DB.QueryRow(`INSERT INTO orgs_org(name, is_active) VALUES($1, TRUE) RETURNING id`, name)
	var id webchat.OrgID
	must(row.Scan(&id))
	return id
}

func InsertChannel(rt *runtime.Runtime, orgID webchat.OrgID, channelType, name, address string, schemes []string) webchat.ChannelUUID {
	uuid := webchat.ChannelUUID(uuids.New())
	_, err := rt.DB.Exec(
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, address, schemes, role, config, log_policy, is_active, created_on, modified_on, created_by_id, modified_by_id) 
		VALUES($1, $2, $3, $4, $5, $6, 'SR', '{}', 'A', TRUE, NOW(), NOW(), 1, 1)`, uuid, orgID, channelType, name, address, pq.Array(schemes),
	)
	noError(err)
	return uuid
}

func InsertContact(rt *runtime.Runtime, orgID webchat.OrgID, name string) webchat.ContactID {
	row := rt.DB.QueryRow(
		`INSERT INTO contacts_contact(uuid, org_id, name, status, ticket_count, is_active, created_on, modified_on) 
		VALUES($1, $2, $3, 'A', 1, TRUE, NOW(), NOW()) RETURNING id`, uuids.New(), orgID, name,
	)
	var id webchat.ContactID
	must(row.Scan(&id))
	return id
}

func InsertURN(rt *runtime.Runtime, orgID webchat.OrgID, contactID webchat.ContactID, urn urns.URN) webchat.URNID {
	scheme, path, _, _ := urn.ToParts()
	row := rt.DB.QueryRow(
		`INSERT INTO contacts_contacturn(org_id, contact_id, scheme, path, identity, priority) 
		VALUES($1, $2, $3, $4, $5, 1000) RETURNING id`, orgID, contactID, scheme, path, urn.Identity(),
	)
	var id webchat.URNID
	must(row.Scan(&id))
	return id
}
