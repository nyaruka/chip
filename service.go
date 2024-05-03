package tembachat

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/redisx"
	"github.com/nyaruka/tembachat/core/courier"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/core/queue"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/web"
	"github.com/nyaruka/tembachat/web/events"
	"github.com/pkg/errors"
)

const (
	outboxTimeLimit = 2 * time.Minute
)

type Service struct {
	rt       *runtime.Runtime
	server   *web.Server
	store    models.Store
	outboxes *queue.Outboxes
	courier  courier.Courier

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
	s.store.Start()

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
	s.store.Stop()

	log.Info("stopped")
}

func (s *Service) Store() models.Store      { return s.store }
func (s *Service) Courier() courier.Courier { return s.courier }

func (s *Service) OnSendRequest(channel *models.Channel, msg *models.MsgOut) {
	log := slog.With("comp", "service")
	rc := s.rt.RP.Get()
	defer rc.Close()

	if err := s.outboxes.AddMessage(rc, channel, msg); err != nil {
		log.Error("error queuing to outbox", "error", err)
	}
}

func (s *Service) sender() {
	defer s.senderWait.Done()
	s.senderWait.Add(1)

	for {
		// TODO panic recovery
		s.send()

		select {
		case <-s.senderStop:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Service) send() {
	log := slog.With("comp", "service")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rc := s.rt.RP.Get()
	defer rc.Close()

	outboxes, err := s.outboxes.All(rc)
	if err != nil {
		log.Error("error reading outboxes", "error", err)
		return
	}

	for _, box := range outboxes {
		ch, err := s.store.GetChannel(ctx, box.ChannelUUID)
		if err != nil {
			log.Error("error fetching channel", "error", err)
			// TODO clear outbox queue ?
			continue
		}

		if time.Since(box.Oldest) > outboxTimeLimit {
			// pop entire outbox and then email or fail
			msgs, err := s.outboxes.PopAll(rc, ch, box.ChatID)
			if err != nil {
				log.Error("error popping all from outbox", "error", err)
			} else if len(msgs) > 0 {
				if err := s.emailOrFail(ctx, ch, box.ChatID, msgs); err != nil {
					log.Error("error handling stalled outbox", "error", err)
				}
			}
		}

		client := s.server.GetClient(box.ChatID)

		if client != nil /*&& client.CanSend()*/ {
			msg, err := s.outboxes.PopMessage(rc, ch, box.ChatID)
			if err != nil {
				log.Error("error popping message from outbox", "error", err)
			} else if msg != nil {
				// TODO find logical place for this so that it can be shared with Client.onCommand
				var user *events.User
				if msg.UserID != models.NilUserID {
					u, err := s.store.GetUser(ctx, msg.UserID)
					if err != nil {
						log.Error("error fetching user", "error", err)
					} else {
						user = events.NewUser(u.Name(), u.Email, u.AvatarURL(s.rt.Config))
					}
				}

				client.Send(events.NewMsgOut(msg.Time, msg.ID, msg.Text, msg.Origin, user))
			}
		}
	}
}

func (s *Service) emailOrFail(ctx context.Context, ch *models.Channel, chatID models.ChatID, msgs []*models.MsgOut) error {
	// TODO load contact, queue messages for email sending, or fail them if no email address
	return nil
}
