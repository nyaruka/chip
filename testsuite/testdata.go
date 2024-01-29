package testsuite

import (
	"github.com/lib/pq"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
)

func InsertOrg(rt *runtime.Runtime, name string) models.OrgID {
	row := rt.DB.QueryRow(`INSERT INTO orgs_org(name, is_active) VALUES($1, TRUE) RETURNING id`, name)
	var id models.OrgID
	must(row.Scan(&id))
	return id
}

func InsertChannel(rt *runtime.Runtime, uuid models.ChannelUUID, orgID models.OrgID, channelType, name, address string, schemes []string) {
	_, err := rt.DB.Exec(
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, address, schemes, role, config, log_policy, is_active, created_on, modified_on, created_by_id, modified_by_id) 
		VALUES($1, $2, $3, $4, $5, $6, 'SR', '{}', 'A', TRUE, NOW(), NOW(), 1, 1)`, uuid, orgID, channelType, name, address, pq.Array(schemes),
	)
	noError(err)
}

func InsertContact(rt *runtime.Runtime, orgID models.OrgID, name string) models.ContactID {
	row := rt.DB.QueryRow(
		`INSERT INTO contacts_contact(uuid, org_id, name, status, ticket_count, is_active, created_on, modified_on) 
		VALUES($1, $2, $3, 'A', 1, TRUE, NOW(), NOW()) RETURNING id`, uuids.New(), orgID, name,
	)
	var id models.ContactID
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

func InsertUser(rt *runtime.Runtime, email, firstName, lastName string) models.UserID {
	row := rt.DB.QueryRow(
		`INSERT INTO auth_user(email, first_name, last_name, is_active, is_staff) 
		VALUES($1, $2, $3, TRUE, FALSE) RETURNING id`, email, firstName, lastName,
	)
	var id models.UserID
	must(row.Scan(&id))
	return id
}
