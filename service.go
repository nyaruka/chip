package tembachat

import (
	"context"
	"log/slog"
	"time"

	"github.com/nyaruka/redisx"
	"github.com/nyaruka/tembachat/core/courier"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/core/queue"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/web"
	"github.com/pkg/errors"
)

type Service struct {
	rt       *runtime.Runtime
	server   *web.Server
	store    models.Store
	outboxes *queue.Outboxes
}

func NewService(cfg *runtime.Config) *Service {
	rt := &runtime.Runtime{Config: cfg}

	s := &Service{
		rt:       rt,
		store:    models.NewStore(rt),
		outboxes: &queue.Outboxes{KeyBase: "chat"},
	}

	s.server = web.NewServer(rt, s)

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

func (s *Service) Store() models.Store { return s.store }

func (s *Service) OnChatStarted(channel models.Channel, contact *models.Contact) {
	log := slog.With("comp", "service")

	if err := courier.NotifyChatStarted(s.rt.Config, channel, contact); err != nil {
		log.Error("error notifying courier", "error", err)
	}
}

func (s *Service) OnChatReceive(channel models.Channel, contact *models.Contact, e events.Event) {
	log := slog.With("comp", "service")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch typed := e.(type) {
	case *events.MsgIn:
		if err := courier.NotifyMsgIn(s.rt.Config, channel, contact, typed); err != nil {
			log.Error("error notifying courier", "error", err)
		}

	case *events.EmailAdded:
		if err := contact.UpdateEmail(ctx, s.rt, typed.Email); err != nil {
			log.Error("error updating email", "error", err)
		}
	}
}

func (s *Service) OnSendRequest(msg *models.MsgOut) {
	// TODO queue message to Redis, let different service instances pick off messages to send via chat or email

	client := s.server.GetClient(msg.ChatID)
	client.Send(events.NewMsgOut(msg.Text, msg.Origin, msg.User))
}
