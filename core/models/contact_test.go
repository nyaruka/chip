package models_test

import (
	"testing"

	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContact(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	twcUUID := testsuite.InsertChannel(rt, orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	channel, err := models.LoadChannel(ctx, rt, twcUUID)
	require.NoError(t, err)

	bob, err := models.LoadContact(ctx, rt, channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.ChatID("65vbbDAQCdPdEWlEhDGy4utO"), bob.ChatID())

	_, err = models.LoadContact(ctx, rt, channel, "123456789012345678901234")
	assert.EqualError(t, err, "contact query returned no rows")
}
