package models_test

import (
	"testing"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	store := models.NewStore(rt)

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows", "")

	// no such channel
	ch, err := store.GetChannel(ctx, "71cdbd54-30c4-4ae6-b122-0a153573d912")
	assert.EqualError(t, err, "sql: no rows in result set")
	assert.Nil(t, ch)

	// from db
	ch, err = store.GetChannel(ctx, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	assert.NoError(t, err)
	assert.Equal(t, models.ChannelUUID("8291264a-4581-4d12-96e5-e9fcfa6e68d9"), ch.UUID)

	// from cache
	ch, err = store.GetChannel(ctx, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	assert.NoError(t, err)
	assert.Equal(t, models.ChannelUUID("8291264a-4581-4d12-96e5-e9fcfa6e68d9"), ch.UUID)

	// no such user
	user, err := store.GetUser(ctx, 345678)
	assert.EqualError(t, err, "sql: no rows in result set")
	assert.Nil(t, user)

	// from db
	user, err = store.GetUser(ctx, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID)

	// from cache
	user, err = store.GetUser(ctx, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID)
}
