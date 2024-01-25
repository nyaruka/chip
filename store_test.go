package tembachat_test

import (
	"testing"

	"github.com/nyaruka/tembachat"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	store := tembachat.NewStore(rt)

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	twcUUID := testsuite.InsertChannel(rt, orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows")

	// no such channel
	ch, err := store.GetChannel(ctx, "8db60a9e-a2aa-4bd3-936b-8c87ba0b16fb")
	assert.EqualError(t, err, "channel query returned no rows")
	assert.Nil(t, ch)

	// from db
	ch, err = store.GetChannel(ctx, twcUUID)
	assert.NoError(t, err)
	assert.Equal(t, twcUUID, ch.UUID())

	// from cache
	ch, err = store.GetChannel(ctx, twcUUID)
	assert.NoError(t, err)
	assert.Equal(t, twcUUID, ch.UUID())

	// no such user
	user, err := store.GetUser(ctx, 345678)
	assert.EqualError(t, err, "user query returned no rows")
	assert.Nil(t, user)

	// from db
	user, err = store.GetUser(ctx, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID())

	// from cache
	user, err = store.GetUser(ctx, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID())
}
