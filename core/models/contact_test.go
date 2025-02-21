package models_test

import (
	"testing"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/gocommon/random"
	"github.com/stretchr/testify/assert"
)

func TestNewChatID(t *testing.T) {
	defer random.SetGenerator(random.DefaultGenerator)
	random.SetGenerator(random.NewSeededGenerator(1234))

	assert.Equal(t, models.ChatID("itlu4O6ZE4ZZc07Y5rHxcLoQ"), models.NewChatID())
	assert.Equal(t, models.ChatID("EMExtx3E2diho8guIXhsfEZb"), models.NewChatID())
	assert.Equal(t, models.ChatID("A0UGLTWLLs59CrFzj6VpvMlG"), models.NewChatID())
}

func TestContact(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"}, map[string]any{"secret": "sesame"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	urnID := testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	// try loading from invalid chat ID
	_, err := models.LoadContact(ctx, rt, orgID, "xyz")
	assert.EqualError(t, err, "invalid path component")

	// try loading from non-existent chat ID
	_, err = models.LoadContact(ctx, rt, orgID, "123456789012345678901234")
	assert.EqualError(t, err, "sql: no rows in result set")

	bob, err := models.LoadContact(ctx, rt, orgID, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, bobID, bob.ID)
	assert.Equal(t, orgID, bob.OrgID)
	assert.Equal(t, urnID, bob.URNID)
	assert.Equal(t, models.ChatID("65vbbDAQCdPdEWlEhDGy4utO"), bob.ChatID)
	assert.Equal(t, "", bob.Email)

	err = bob.UpdateEmail(ctx, rt, "bob@nyaruka.com")
	assert.NoError(t, err)
	assert.Equal(t, "bob@nyaruka.com", bob.Email)

	bob, err = models.LoadContact(ctx, rt, orgID, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.ChatID("65vbbDAQCdPdEWlEhDGy4utO"), bob.ChatID)
	assert.Equal(t, "bob@nyaruka.com", bob.Email)
}
