package tembachat

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

// Cache is a generic cache using ttlcache.Cache but adding a fetch function which takes context and can error. Could
// move to gocommon.
type Cache[K comparable, V any] struct {
	cache *ttlcache.Cache[K, V]
	fetch FetchFunc[K, V]
}

type FetchFunc[K comparable, V any] func(context.Context, K) (V, error)

func NewCache[K comparable, V any](fetch FetchFunc[K, V], ttl time.Duration) *Cache[K, V] {
	c := ttlcache.New[K, V](
		ttlcache.WithTTL[K, V](ttl),
		ttlcache.WithDisableTouchOnHit[K, V](),
	)
	go c.Start()

	return &Cache[K, V]{cache: c, fetch: fetch}
}

func (c *Cache[K, V]) Stop() {
	c.cache.Stop()
}

func (c *Cache[K, V]) Len() int {
	return c.cache.Len()
}

func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	item := c.cache.Get(key)

	if item == nil {
		var v V
		var err error

		// we don't prevent multiple simultaneous fetches for same key but that's preferable to over locking on hits
		v, err = c.fetch(ctx, key)
		if err != nil {
			return v, err
		}

		item = c.cache.Set(key, v, ttlcache.DefaultTTL)
	}

	return item.Value(), nil
}

type Store interface {
	GetChannel(context.Context, webchat.ChannelUUID) (webchat.Channel, error)
	GetUser(context.Context, string) (webchat.User, error)
	Close()
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *Cache[webchat.ChannelUUID, webchat.Channel]
	users    *Cache[string, webchat.User]
}

func NewStore(rt *runtime.Runtime) Store {
	fetchChannel := func(ctx context.Context, uuid webchat.ChannelUUID) (webchat.Channel, error) {
		return webchat.LoadChannel(ctx, rt, uuid)
	}
	fetchUser := func(ctx context.Context, email string) (webchat.User, error) {
		return webchat.LoadUser(ctx, rt, email)
	}

	return &store{
		rt:       rt,
		channels: NewCache[webchat.ChannelUUID, webchat.Channel](fetchChannel, 30*time.Second),
		users:    NewCache[string, webchat.User](fetchUser, 30*time.Second),
	}
}

func (s *store) GetChannel(ctx context.Context, uuid webchat.ChannelUUID) (webchat.Channel, error) {
	return s.channels.Get(ctx, uuid)
}

func (s *store) GetUser(ctx context.Context, email string) (webchat.User, error) {
	return s.users.Get(ctx, email)
}

func (s *store) Close() {
	s.channels.Stop()
	s.users.Stop()
}
