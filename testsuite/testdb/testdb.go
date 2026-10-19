// Package testdb has helpers for putting webchat channels, visitors and their conversations into a test database,
// on top of the workspaces and users the dump seeds it with.
package testdb

import (
	"context"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/stretchr/testify/require"
)

// the workspaces in the dump
const (
	Org1 models.OrgID = 1
	Org2 models.OrgID = 2
)

// InsertChannel inserts an active webchat channel with the given UUID and config into the workspace
func InsertChannel(t *testing.T, rt *runtime.Runtime, orgID models.OrgID, uuid models.ChannelUUID, config map[string]any) *models.Channel {
	t.Helper()

	if config == nil {
		config = map[string]any{}
	}

	_, err := rt.DB.ExecContext(context.Background(),
		`INSERT INTO channels_channel(uuid, org_id, channel_type, name, schemes, address, config, role, is_active, is_system, is_enabled, created_on, modified_on, created_by_id, modified_by_id)
		      VALUES($1, $2, $3, 'WebChat', '{webchat}', '', $4, 'SR', TRUE, FALSE, TRUE, NOW(), NOW(), $5, $5)`,
		uuid, orgID, models.ChannelTypeWebChat, jsonx.MustMarshal(config), systemUserID(t, rt),
	)
	require.NoError(t, err)

	ch, err := models.LoadChannel(context.Background(), rt.DB, uuid)
	require.NoError(t, err)
	require.NotNil(t, ch)
	return ch
}

// DeactivateChannel deactivates the given channel, as releasing it on the platform does
func DeactivateChannel(t *testing.T, rt *runtime.Runtime, ch *models.Channel) {
	t.Helper()

	_, err := rt.DB.ExecContext(context.Background(), `UPDATE channels_channel SET is_active = FALSE WHERE id = $1`, ch.ID())
	require.NoError(t, err)
}

// InsertVisitor inserts a contact chatting on the given channel under the given chat ID, the way courier creates
// one when a chat starts: an active contact in the channel's workspace with a webchat URN whose path is the chat
// ID and whose channel is the chat's
func InsertVisitor(t *testing.T, rt *runtime.Runtime, ch *models.Channel, chatID string) *models.Visitor {
	t.Helper()

	ctx := context.Background()

	urn, err := urns.NewFromParts(urns.WebChat.Prefix, chatID, nil, "")
	require.NoError(t, err)

	var contactID models.ContactID
	err = rt.DB.GetContext(ctx, &contactID,
		`INSERT INTO contacts_contact(org_id, is_active, status, uuid, created_on, modified_on, created_by_id, modified_by_id, name, ticket_count)
		      VALUES($1, TRUE, 'A', $2, NOW(), NOW(), $3, $3, NULL, 0) RETURNING id`,
		ch.OrgID(), uuids.NewV4(), systemUserID(t, rt),
	)
	require.NoError(t, err)

	_, err = rt.DB.ExecContext(ctx,
		`INSERT INTO contacts_contacturn(org_id, identity, path, scheme, priority, channel_id, contact_id)
		      VALUES($1, $2, $3, $4, 1000, $5, $6)`,
		ch.OrgID(), urn.Identity(), urn.Path(), urn.Scheme(), ch.ID(), contactID,
	)
	require.NoError(t, err)

	visitor, err := models.LoadVisitor(ctx, rt.DB, ch, chatID)
	require.NoError(t, err)
	require.NotNil(t, visitor)
	return visitor
}

// ReleaseContact deactivates the given visitor's contact, as releasing them on the platform does
func ReleaseContact(t *testing.T, rt *runtime.Runtime, visitor *models.Visitor) {
	t.Helper()

	_, err := rt.DB.ExecContext(context.Background(), `UPDATE contacts_contact SET is_active = FALSE WHERE id = $1`, visitor.ContactID)
	require.NoError(t, err)
}

// MsgOptions is what a test can vary about a message
type MsgOptions struct {
	Attachments  []string
	QuickReplies string // JSON
	Visibility   string // default V
	Status       string // default W for outgoing, P for incoming
	SentBy       models.UserID
}

// InsertMsg inserts a message in the given visitor's conversation on the given channel
func InsertMsg(t *testing.T, rt *runtime.Runtime, ch *models.Channel, visitor *models.Visitor, uuid models.MsgUUID, direction models.MsgDirection, text string, createdOn time.Time, opts MsgOptions) {
	t.Helper()

	visibility := opts.Visibility
	if visibility == "" {
		visibility = "V"
	}
	status := opts.Status
	if status == "" {
		if direction == models.MsgOutgoing {
			status = "W"
		} else {
			status = "P"
		}
	}
	var quickReplies *string
	if opts.QuickReplies != "" {
		quickReplies = &opts.QuickReplies
	}
	var sentBy *models.UserID
	if opts.SentBy != 0 {
		sentBy = &opts.SentBy
	}

	// the platform files incoming messages in the inbox and outgoing ones as sent, which is when they went out
	folder := "I"
	var sentOn *time.Time
	if direction == models.MsgOutgoing {
		folder = "S"
		sentOn = &createdOn
	}

	_, err := rt.DB.ExecContext(context.Background(),
		`INSERT INTO msgs_msg(uuid, org_id, channel_id, contact_id, contact_urn_id, text, attachments, quickreplies, created_on, modified_on, direction, status, visibility, msg_type, is_android, high_priority, msg_count, error_count, created_by_id, folder, sent_on)
		      VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $11, $12, 'T', FALSE, FALSE, 1, 0, $13, $14, $15)`,
		uuid, ch.OrgID(), ch.ID(), visitor.ContactID, visitor.URNID, text, pq.StringArray(opts.Attachments), quickReplies, createdOn, direction, status, visibility, sentBy, folder, sentOn,
	)
	require.NoError(t, err)
}

// UserID returns the id of the user in the dump with the given email
func UserID(t *testing.T, rt *runtime.Runtime, email string) models.UserID {
	t.Helper()

	var id models.UserID
	require.NoError(t, rt.DB.GetContext(context.Background(), &id, `SELECT id FROM users_user WHERE email = $1`, email))
	return id
}

// SetAvatar gives the given user an avatar in public storage
func SetAvatar(t *testing.T, rt *runtime.Runtime, userID models.UserID, path string) {
	t.Helper()

	_, err := rt.DB.ExecContext(context.Background(), `UPDATE users_user SET avatar = $2 WHERE id = $1`, userID, path)
	require.NoError(t, err)
}

func systemUserID(t *testing.T, rt *runtime.Runtime) models.UserID {
	t.Helper()

	var id models.UserID
	require.NoError(t, rt.DB.GetContext(context.Background(), &id, `SELECT id FROM users_user WHERE is_system`))
	return id
}
