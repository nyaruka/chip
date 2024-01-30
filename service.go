package tembachat

import (
	"context"
	"log/slog"
	"sync"
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

	senderStop chan bool
	senderWait sync.WaitGroup
}

func NewService(cfg *runtime.Config) *Service {
	rt := &runtime.Runtime{Config: cfg}

	s := &Service{
		rt:         rt,
		store:      models.NewStore(rt),
		outboxes:   &queue.Outboxes{KeyBase: "chat"},
		senderStop: make(chan bool),
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

	go s.sender()

	log.Info("started")
	return nil
}

func (s *Service) Stop() {
	log := slog.With("comp", "service")
	log.Info("stopping...")

	s.senderStop <- true
	s.senderWait.Wait()

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
	log := slog.With("comp", "service")
	rc := s.rt.RP.Get()
	defer rc.Close()

	if err := s.outboxes.AddMessage(rc, msg); err != nil {
		log.Error("error queuing to outbox", "error", err)
	}
}

func (s *Service) sender() {
	defer s.senderWait.Done()
	s.senderWait.Add(1)

	for {
		s.send()

		select {
		case <-s.senderStop:
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *Service) send() {
	log := slog.With("comp", "service")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rc := s.rt.RP.Get()
	defer rc.Close()

	chatIDs, err := s.outboxes.Boxes(rc)
	if err != nil {
		log.Error("error reading outboxes", "error", err)
		return
	}

	for _, chatID := range chatIDs {
		client := s.server.GetClient(chatID)
		if client != nil && client.CanSend() {
			msg, err := s.outboxes.PopMessage(rc, chatID)
			if err != nil {
				log.Error("error popping message from outbox", "error", err)
			} else if msg != nil {
				var user models.User
				if msg.UserID != models.NilUserID {
					user, err = s.store.GetUser(ctx, msg.UserID)
					if err != nil {
						log.Error("error fetching user", "error", err)
					}
				}

				client.Send(events.NewMsgOut(msg.Text, msg.Origin, user))
			}
		}
	}
}
