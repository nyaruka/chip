package models_test

import (
	"testing"
	"time"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadChannel(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)

	testdb.InsertChannel(t, rt, testdb.Org1, "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71", map[string]any{"allowed_domains": []string{"example.com", "localhost:3000"}})
	plain := testdb.InsertChannel(t, rt, testdb.Org2, "b81c3f45-2d6e-4a1f-9c72-8e5d0a4b6f13", nil)

	ch, err := models.LoadChannel(ctx, rt.DB, "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71")
	require.NoError(t, err)
	assert.Equal(t, models.ChannelUUID("0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71"), ch.UUID())
	assert.Equal(t, testdb.Org1, ch.OrgID())
	assert.Equal(t, []string{"example.com", "localhost:3000"}, ch.AllowedDomains())

	ch, err = models.LoadChannel(ctx, rt.DB, plain.UUID())
	require.NoError(t, err)
	assert.Equal(t, testdb.Org2, ch.OrgID())
	assert.Empty(t, ch.AllowedDomains())

	// channels that don't exist, aren't active or aren't webchat channels aren't loaded
	ch, err = models.LoadChannel(ctx, rt.DB, "11111111-2222-3333-4444-555555555555")
	require.NoError(t, err)
	assert.Nil(t, ch)

	testdb.DeactivateChannel(t, rt, plain)
	ch, err = models.LoadChannel(ctx, rt.DB, plain.UUID())
	require.NoError(t, err)
	assert.Nil(t, ch)

	rt.DB.MustExec(`UPDATE channels_channel SET is_active = TRUE, channel_type = 'EX' WHERE uuid = $1`, plain.UUID())
	ch, err = models.LoadChannel(ctx, rt.DB, plain.UUID())
	require.NoError(t, err)
	assert.Nil(t, ch)

	assert.Equal(t, "chat:0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71:vM0GGhDrqpTQefIEinK0up3C", models.ChatSocket("0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71", "vM0GGhDrqpTQefIEinK0up3C"))
}

func TestLoadVisitor(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)

	ch := testdb.InsertChannel(t, rt, testdb.Org1, "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71", nil)
	other := testdb.InsertChannel(t, rt, testdb.Org1, "b81c3f45-2d6e-4a1f-9c72-8e5d0a4b6f13", nil)
	inserted := testdb.InsertVisitor(t, rt, ch, "vM0GGhDrqpTQefIEinK0up3C")

	visitor, err := models.LoadVisitor(ctx, rt.DB, ch, "vM0GGhDrqpTQefIEinK0up3C")
	require.NoError(t, err)
	assert.Equal(t, inserted, visitor)

	// a chat ID isn't known on another channel, nor is one we never minted or that isn't the right shape
	for _, tc := range []struct {
		ch     *models.Channel
		chatID string
	}{
		{other, "vM0GGhDrqpTQefIEinK0up3C"},
		{ch, "xxxxxhDrqpTQefIEinK0up3C"},
		{ch, "not-a-chat-id!"},
		{ch, ""},
	} {
		visitor, err = models.LoadVisitor(ctx, rt.DB, tc.ch, tc.chatID)
		require.NoError(t, err)
		assert.Nil(t, visitor, tc.chatID)
	}

	// and a released contact's chat is gone
	testdb.ReleaseContact(t, rt, inserted)
	visitor, err = models.LoadVisitor(ctx, rt.DB, ch, "vM0GGhDrqpTQefIEinK0up3C")
	require.NoError(t, err)
	assert.Nil(t, visitor)
}

func TestTextQuickReplies(t *testing.T) {
	qrs := []models.QuickReply{{Type: "text", Text: "Yes"}, {Type: "url", Text: "More", Extra: "https://example.com"}, {Type: "text", Text: "No"}}
	assert.Equal(t, []models.QuickReply{{Type: "text", Text: "Yes"}, {Type: "text", Text: "No"}}, models.TextQuickReplies(qrs))
	assert.Nil(t, models.TextQuickReplies(nil))
}

func TestChannelEmailConfig(t *testing.T) {
	config := func(kvs ...string) models.ChannelConfig {
		c := models.ChannelConfig{}
		for i := 0; i < len(kvs); i += 2 {
			c[kvs[i]] = kvs[i+1]
		}
		return c
	}

	ch := &models.Channel{Config_: config("email_from", "help@example.com", "mailgun_domain", "mg.example.com", "mailgun_region", "eu", "mailgun_api_key", "key-1")}
	assert.Equal(t, &models.EmailConfig{From: "help@example.com", Domain: "mg.example.com", Region: "eu", APIKey: "key-1"}, ch.EmailConfig())

	// email is off without a domain, and a channel missing any other setting can't send either
	for _, c := range []models.ChannelConfig{
		{},
		config("email_from", "help@example.com", "mailgun_domain", "", "mailgun_region", "us", "mailgun_api_key", "key-1"),
		config("email_from", "", "mailgun_domain", "mg.example.com", "mailgun_region", "us", "mailgun_api_key", "key-1"),
		config("email_from", "help@example.com", "mailgun_domain", "mg.example.com", "mailgun_region", "us", "mailgun_api_key", ""),
		{"email_from": "help@example.com", "mailgun_domain": "mg.example.com", "mailgun_region": "us", "mailgun_api_key": 123},
		{"email_from": []any{"help@example.com"}, "mailgun_domain": "mg.example.com", "mailgun_region": "us", "mailgun_api_key": "key-1"},
	} {
		ch := &models.Channel{Config_: c}
		assert.Nil(t, ch.EmailConfig(), c)
	}
}

func TestLoadOrg(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)

	org, err := models.LoadOrg(ctx, rt.DB, testdb.Org2)
	require.NoError(t, err)
	assert.Equal(t, testdb.Org2, org.ID())
	assert.Equal(t, "Nyaruka", org.Name())
	assert.Equal(t, "America/Los_Angeles", org.Timezone().String())

	// a timezone we don't know is shown as UTC
	org.Timezone_ = "Mars/Olympus_Mons"
	assert.Equal(t, time.UTC, org.Timezone())

	// workspaces that don't exist or aren't active aren't loaded
	org, err = models.LoadOrg(ctx, rt.DB, 12345)
	require.NoError(t, err)
	assert.Nil(t, org)

	rt.DB.MustExec(`UPDATE orgs_org SET is_active = FALSE WHERE id = $1`, testdb.Org2)
	org, err = models.LoadOrg(ctx, rt.DB, testdb.Org2)
	require.NoError(t, err)
	assert.Nil(t, org)
}

func TestGetUnreadChatMsgs(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)

	ch := testdb.InsertChannel(t, rt, testdb.Org1, "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71", nil)
	visitor := testdb.InsertVisitor(t, rt, ch, "vM0GGhDrqpTQefIEinK0up3C")
	otherVisitor := testdb.InsertVisitor(t, rt, ch, "xA9GGhDrqpTQefIEinK0up3C")
	adminID := testdb.UserID(t, rt, "admin1@textit.com")
	createdOn := time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC)

	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd51", models.MsgOutgoing, "Hi", createdOn, testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd52", models.MsgOutgoing, "Read", createdOn, testdb.MsgOptions{Status: "R"})
	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd53", models.MsgOutgoing, "Deleted", createdOn, testdb.MsgOptions{Visibility: "X"})
	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd54", models.MsgIncoming, "Incoming", createdOn, testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, otherVisitor, "0191e180-7d60-7000-aded-7d8b151cbd55", models.MsgOutgoing, "Not theirs", createdOn, testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd56", models.MsgOutgoing, "From Andy", createdOn, testdb.MsgOptions{SentBy: adminID, Status: "D"})
	testdb.InsertMsg(t, rt, ch, visitor, "0191e180-7d60-7000-aded-7d8b151cbd57", models.MsgOutgoing, "Not asked for", createdOn, testdb.MsgOptions{})

	msgs, err := models.GetUnreadChatMsgs(ctx, rt.DB, ch, visitor, []models.MsgUUID{
		"0191e180-7d60-7000-aded-7d8b151cbd56",
		"0191e180-7d60-7000-aded-7d8b151cbd51",
		"0191e180-7d60-7000-aded-7d8b151cbd52",
		"0191e180-7d60-7000-aded-7d8b151cbd53",
		"0191e180-7d60-7000-aded-7d8b151cbd54",
		"0191e180-7d60-7000-aded-7d8b151cbd55",
	})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, models.MsgUUID("0191e180-7d60-7000-aded-7d8b151cbd51"), msgs[0].UUID)
	assert.Nil(t, msgs[0].Sender())
	assert.Equal(t, models.MsgUUID("0191e180-7d60-7000-aded-7d8b151cbd56"), msgs[1].UUID)
	assert.Equal(t, "Andy Admin", msgs[1].Sender().Name())

	msgs, err = models.GetUnreadChatMsgs(ctx, rt.DB, ch, visitor, nil)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}
