package models_test

import (
	"testing"

	"github.com/nyaruka/tembachat/testsuite"
	"github.com/nyaruka/tembachat/webchat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURNExists(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	twcUUID := testsuite.InsertChannel(rt, orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	channel, err := models.LoadChannel(ctx, rt, twcUUID)
	require.NoError(t, err)

	exists, err := models.URNExists(ctx, rt, channel, "webchat:65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = models.URNExists(ctx, rt, channel, "webchat:123456789012345678901234")
	assert.NoError(t, err)
	assert.False(t, exists)
}
