package tembachat_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nyaruka/tembachat"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	ctx := context.Background()

	fetches := 0
	fetch := func(ctx context.Context, s string) (string, error) {
		fetches += 1
		if s == "e" {
			return "", errors.New("boom")
		}
		return strings.ToUpper(s), nil
	}
	cache := tembachat.NewCache[string, string](fetch, 500*time.Millisecond)

	v, err := cache.Get(ctx, "x")
	assert.NoError(t, err)
	assert.Equal(t, "X", v)
	assert.Equal(t, 1, fetches)

	v, err = cache.Get(ctx, "x")
	assert.NoError(t, err)
	assert.Equal(t, "X", v)
	assert.Equal(t, 1, fetches)

	v, err = cache.Get(ctx, "y")
	assert.NoError(t, err)
	assert.Equal(t, "Y", v)
	assert.Equal(t, 2, fetches)

	v, err = cache.Get(ctx, "e")
	assert.EqualError(t, err, "boom")
	assert.Equal(t, "", v)
	assert.Equal(t, 3, fetches)

	assert.Equal(t, 2, cache.Len())

	time.Sleep(time.Second)

	assert.Equal(t, 0, cache.Len())

	cache.Stop()
}

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
	user, err := store.GetUser(ctx, "jim@nyaruka.com")
	assert.EqualError(t, err, "user query returned no rows")
	assert.Nil(t, user)

	// from db
	user, err = store.GetUser(ctx, "bob@nyaruka.com")
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID())

	// from cache
	user, err = store.GetUser(ctx, "bob@nyaruka.com")
	assert.NoError(t, err)
	assert.Equal(t, bobID, user.ID())
}
