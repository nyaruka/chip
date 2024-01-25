package models

import (
	"context"
	"time"

	"github.com/nyaruka/gocommon/cache"
	"github.com/nyaruka/tembachat/runtime"
)

type Store interface {
	GetChannel(context.Context, ChannelUUID) (Channel, error)
	GetUser(context.Context, UserID) (User, error)
	Close()
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *cache.Local[ChannelUUID, Channel]
	users    *cache.Local[UserID, User]
}

func NewStore(rt *runtime.Runtime) Store {
	fetchChannel := func(ctx context.Context, uuid ChannelUUID) (Channel, error) {
		return LoadChannel(ctx, rt, uuid)
	}
	fetchUser := func(ctx context.Context, id UserID) (User, error) {
		return LoadUser(ctx, rt, id)
	}

	return &store{
		rt:       rt,
		channels: cache.NewLocal[ChannelUUID, Channel](fetchChannel, 30*time.Second),
		users:    cache.NewLocal[UserID, User](fetchUser, 30*time.Second),
	}
}

func (s *store) GetChannel(ctx context.Context, uuid ChannelUUID) (Channel, error) {
	return s.channels.GetOrFetch(ctx, uuid)
}

func (s *store) GetUser(ctx context.Context, id UserID) (User, error) {
	return s.users.GetOrFetch(ctx, id)
}

func (s *store) Close() {
	s.channels.Stop()
	s.users.Stop()
}
