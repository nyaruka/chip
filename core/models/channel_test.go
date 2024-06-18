package models_test

import (
	"testing"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestLoadChannel(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "Web Chat", "", []string{"webchat"}, map[string]any{"secret": "sesame"})

	_, err := models.LoadChannel(ctx, rt, "ecf5ff5d-0c2d-4850-8641-e3f2fc7afaea")
	assert.EqualError(t, err, "sql: no rows in result set")

	ch, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	assert.NoError(t, err)
	assert.Equal(t, models.ChannelUUID("8291264a-4581-4d12-96e5-e9fcfa6e68d9"), ch.UUID)
	assert.Equal(t, orgID, ch.OrgID)
	assert.Equal(t, "sesame", ch.Secret())
}
