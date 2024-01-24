package tembachat

import (
	"context"
	"time"

	"github.com/nyaruka/gocommon/cache"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

type Store interface {
	GetChannel(context.Context, webchat.ChannelUUID) (webchat.Channel, error)
	GetUser(context.Context, string) (webchat.User, error)
	Close()
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *cache.Cache[webchat.ChannelUUID, webchat.Channel]
	users    *cache.Cache[string, webchat.User]
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
		channels: cache.NewCache[webchat.ChannelUUID, webchat.Channel](fetchChannel, 30*time.Second),
		users:    cache.NewCache[string, webchat.User](fetchUser, 30*time.Second),
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
