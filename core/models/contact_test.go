package models_test

import (
	"testing"

	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContact(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	channel, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	require.NoError(t, err)

	// try loading from invalid chat ID
	_, err = models.LoadContact(ctx, rt, channel, "xyz")
	assert.EqualError(t, err, "invalid webchat id: xyz")

	// try loading from non-existent chat ID
	_, err = models.LoadContact(ctx, rt, channel, "123456789012345678901234")
	assert.EqualError(t, err, "contact query returned no rows")

	bob, err := models.LoadContact(ctx, rt, channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.ChatID("65vbbDAQCdPdEWlEhDGy4utO"), bob.ChatID)
	assert.Equal(t, "", bob.Email)

	err = bob.UpdateEmail(ctx, rt, "bob@nyaruka.com")
	assert.NoError(t, err)
	assert.Equal(t, "bob@nyaruka.com", bob.Email)

	bob, err = models.LoadContact(ctx, rt, channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.ChatID("65vbbDAQCdPdEWlEhDGy4utO"), bob.ChatID)
	assert.Equal(t, "bob@nyaruka.com", bob.Email)
}
