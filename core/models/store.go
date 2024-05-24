package models

import (
	"context"
	"log/slog"
	"time"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/cache"
)

type Store interface {
	Start()
	Stop()
	GetChannel(context.Context, ChannelUUID) (*Channel, error)
	GetUser(context.Context, UserID) (*User, error)
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *cache.Local[ChannelUUID, *Channel]
	users    *cache.Local[UserID, *User]
}

func NewStore(rt *runtime.Runtime) Store {
	fetchChannel := func(ctx context.Context, uuid ChannelUUID) (*Channel, error) {
		return LoadChannel(ctx, rt, uuid)
	}
	fetchUser := func(ctx context.Context, id UserID) (*User, error) {
		return LoadUser(ctx, rt, id)
	}

	return &store{
		rt:       rt,
		channels: cache.NewLocal(fetchChannel, 30*time.Second),
		users:    cache.NewLocal(fetchUser, 30*time.Second),
	}
}

func (s *store) Start() {
	s.channels.Start()
	s.users.Start()

	slog.With("comp", "store").Info("started")
}

func (s *store) Stop() {
	s.channels.Stop()
	s.users.Stop()

	slog.With("comp", "store").Info("stopped")
}

func (s *store) GetChannel(ctx context.Context, uuid ChannelUUID) (*Channel, error) {
	return s.channels.GetOrFetch(ctx, uuid)
}

func (s *store) GetUser(ctx context.Context, id UserID) (*User, error) {
	return s.users.GetOrFetch(ctx, id)
}
