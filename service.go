package tembachat

import (
	"log/slog"

	"github.com/nyaruka/redisx"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/web"
	"github.com/pkg/errors"
)

type Service struct {
	rt     *runtime.Runtime
	server *web.Server
	store  models.Store
}

func NewService(cfg *runtime.Config) *Service {
	rt := &runtime.Runtime{Config: cfg}
	store := models.NewStore(rt)

	return &Service{
		rt:     rt,
		server: web.NewServer(rt, store),
		store:  store,
	}
}

func (s *Service) Start() error {
	log := slog.With("comp", "service")
	var err error

	s.rt.DB, err = runtime.OpenDBPool(s.rt.Config.DB, 16)
	if err != nil {
		return errors.Wrapf(err, "error connecting to database")
	} else {
		log.Info("db ok")
	}

	s.rt.RP, err = redisx.NewPool(s.rt.Config.Redis)
	if err != nil {
		return errors.Wrapf(err, "error connecting to redis")
	} else {
		log.Info("redis ok")
	}

	s.server.Start()

	log.Info("started")
	return nil
}

func (s *Service) Stop() {
	log := slog.With("comp", "service")
	log.Info("stopping...")

	s.server.Stop()

	s.store.Close()

	log.Info("stopped")
}
