package tembachat

import (
	"context"
	"time"

	"github.com/nyaruka/gocommon/cache"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat/models"
)

type Store interface {
	GetChannel(context.Context, models.ChannelUUID) (models.Channel, error)
	GetUser(context.Context, models.UserID) (models.User, error)
	Close()
}

// implementation of Store using cached database lookups
type store struct {
	rt       *runtime.Runtime
	channels *cache.Local[models.ChannelUUID, models.Channel]
	users    *cache.Local[models.UserID, models.User]
}

func NewStore(rt *runtime.Runtime) Store {
	fetchChannel := func(ctx context.Context, uuid models.ChannelUUID) (models.Channel, error) {
		return models.LoadChannel(ctx, rt, uuid)
	}
	fetchUser := func(ctx context.Context, id models.UserID) (models.User, error) {
		return models.LoadUser(ctx, rt, id)
	}

	return &store{
		rt:       rt,
		channels: cache.NewLocal[models.ChannelUUID, models.Channel](fetchChannel, 30*time.Second),
		users:    cache.NewLocal[models.UserID, models.User](fetchUser, 30*time.Second),
	}
}

func (s *store) GetChannel(ctx context.Context, uuid models.ChannelUUID) (models.Channel, error) {
	return s.channels.GetOrFetch(ctx, uuid)
}

func (s *store) GetUser(ctx context.Context, id models.UserID) (models.User, error) {
	return s.users.GetOrFetch(ctx, id)
}

func (s *store) Close() {
	s.channels.Stop()
	s.users.Stop()
}
