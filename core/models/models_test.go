package models_test

import (
	"testing"

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
