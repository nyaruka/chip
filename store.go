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
	GetUser(context.Context, webchat.UserID) (webchat.User, error)
	Close()
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *cache.Local[webchat.ChannelUUID, webchat.Channel]
	users    *cache.Local[webchat.UserID, webchat.User]
}

func NewStore(rt *runtime.Runtime) Store {
	fetchChannel := func(ctx context.Context, uuid webchat.ChannelUUID) (webchat.Channel, error) {
		return webchat.LoadChannel(ctx, rt, uuid)
	}
	fetchUser := func(ctx context.Context, id webchat.UserID) (webchat.User, error) {
		return webchat.LoadUser(ctx, rt, id)
	}

	return &store{
		rt:       rt,
		channels: cache.NewLocal[webchat.ChannelUUID, webchat.Channel](fetchChannel, 30*time.Second),
		users:    cache.NewLocal[webchat.UserID, webchat.User](fetchUser, 30*time.Second),
	}
}

func (s *store) GetChannel(ctx context.Context, uuid webchat.ChannelUUID) (webchat.Channel, error) {
	return s.channels.GetOrFetch(ctx, uuid)
}

func (s *store) GetUser(ctx context.Context, id webchat.UserID) (webchat.User, error) {
	return s.users.GetOrFetch(ctx, id)
}

func (s *store) Close() {
	s.channels.Stop()
	s.users.Stop()
}
