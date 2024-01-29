package tembachat

import (
	"log/slog"

	"github.com/nyaruka/redisx"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/courier"
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

	s := &Service{rt: rt, store: store}

	s.server = web.NewServer(rt, store, s.handleSendRequest, s.handleChatReceived)

	return s
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

func (s *Service) handleSendRequest(msg *models.MsgOut) {
	// TODO queue message to Redis, let different service instances pick off messages to send via chat or email

	client := s.server.GetClient(msg.Contact.ChatID())
	client.Send(events.NewMsgOut(msg.Text, msg.Origin, msg.User))
}

func (s *Service) handleChatReceived(c *web.Client, e events.Event) {
	switch e.(type) {
	case *events.ChatStarted, *events.MsgIn:
		courier.Notify(s.rt.Config, c.Channel(), c.Contact(), e)

	case *events.EmailAdded:
		// TODO update URN? add new URN?
	}
}
