package testsuite

import (
	"time"

	"github.com/lib/pq"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/null/v2"
)

func InsertOrg(rt *runtime.Runtime, name string) models.OrgID {
	row := rt.DB.QueryRow(`INSERT INTO orgs_org(name, is_active) VALUES($1, TRUE) RETURNING id`, name)
	var id models.OrgID
	must(row.Scan(&id))
	return id
}

func InsertChannel(rt *runtime.Runtime, uuid models.ChannelUUID, orgID models.OrgID, channelType, name, address string, schemes []string, config map[string]any) models.ChannelID {
	row := rt.DB.QueryRow(
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, address, schemes, role, config, log_policy, is_active, created_on, modified_on, created_by_id, modified_by_id) 
		VALUES($1, $2, $3, $4, $5, $6, 'SR', $7, 'A', TRUE, NOW(), NOW(), 1, 1) RETURNING id`, uuid, orgID, channelType, name, address, pq.Array(schemes), null.Map(config),
	)
	var id models.ChannelID
	must(row.Scan(&id))
	return id
}

func InsertContact(rt *runtime.Runtime, orgID models.OrgID, name string) models.ContactID {
	row := rt.DB.QueryRow(
		`INSERT INTO contacts_contact(uuid, org_id, name, status, ticket_count, is_active, created_on, modified_on) 
		VALUES($1, $2, $3, 'A', 1, TRUE, NOW(), NOW()) RETURNING id`, uuids.NewV4(), orgID, name,
	)
	var id models.ContactID
	must(row.Scan(&id))
	return id
}

func InsertIncomingMsg(rt *runtime.Runtime, orgID models.OrgID, channelID models.ChannelID, contactID models.ContactID, urnID models.URNID, text string, createdOn time.Time) models.MsgID {
	row := rt.DB.QueryRow(
		`INSERT INTO msgs_msg(uuid, org_id, channel_id, contact_id, contact_urn_id, direction, msg_type, status, visibility, text, created_on, modified_on, next_attempt, msg_count, error_count)
	  	 VALUES($1, $2, $3, $4, $5, 'I', 'T', 'H', 'V', $6, $7, NOW(), NOW(), 1, 1) RETURNING id`, uuids.NewV4(), orgID, channelID, contactID, urnID, text, createdOn,
	)
	var id models.MsgID
	must(row.Scan(&id))
	return id
}

func InsertOutgoingMsg(rt *runtime.Runtime, orgID models.OrgID, channelID models.ChannelID, contactID models.ContactID, urnID models.URNID, text string, createdOn time.Time) models.MsgID {
	row := rt.DB.QueryRow(
		`INSERT INTO msgs_msg(uuid, org_id, channel_id, contact_id, contact_urn_id, direction, msg_type, status, visibility, text, created_on, modified_on, next_attempt, msg_count, error_count)
	  	 VALUES($1, $2, $3, $4, $5, 'O', 'T', 'Q', 'V', $6, $7, NOW(), NOW(), 1, 1) RETURNING id`, uuids.NewV4(), orgID, channelID, contactID, urnID, text, createdOn,
	)
	var id models.MsgID
	must(row.Scan(&id))
	return id
}

func InsertURN(rt *runtime.Runtime, orgID models.OrgID, contactID models.ContactID, urn urns.URN) models.URNID {
	scheme, path, _, display := urn.ToParts()
	row := rt.DB.QueryRow(
		`INSERT INTO contacts_contacturn(org_id, contact_id, scheme, path, identity, display, priority) 
		VALUES($1, $2, $3, $4, $5, $6, 1000) RETURNING id`, orgID, contactID, scheme, path, urn.Identity(), display,
	)
	var id models.URNID
	must(row.Scan(&id))
	return id
}

func InsertUser(rt *runtime.Runtime, email, firstName, lastName, avatar string) models.UserID {
	row := rt.DB.QueryRow(
		`INSERT INTO users_user(email, first_name, last_name, is_active, is_staff, avatar) 
		VALUES($1, $2, $3, TRUE, FALSE, $4) RETURNING id`, email, firstName, lastName, avatar,
	)
	var id models.UserID
	must(row.Scan(&id))
	return id
}
