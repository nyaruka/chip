package webchat_test

import (
	"testing"

	"github.com/nyaruka/tembachat/testsuite"
	"github.com/nyaruka/tembachat/webchat"
	"github.com/stretchr/testify/assert"
)

func TestLoadChannel(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	twcUUID := testsuite.InsertChannel(rt, orgID, "TWC", "WebChat", "123", []string{"webchat"})

	_, err := webchat.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	assert.EqualError(t, err, "channel query returned no rows")

	ch, err := webchat.LoadChannel(ctx, rt, twcUUID)
	assert.NoError(t, err)
	assert.Equal(t, twcUUID, ch.UUID())
}
