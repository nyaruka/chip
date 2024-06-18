package chip

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/chip/core/courier"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/core/queue"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/chip/web"
	"github.com/nyaruka/chip/web/events"
)

type Service struct {
	rt      *runtime.Runtime
	server  *web.Server
	store   models.Store
	outbox  *queue.Outbox
	courier courier.Courier

	senderStop chan bool
	senderWait sync.WaitGroup
}

func NewService(rt *runtime.Runtime, courier courier.Courier) *Service {
	s := &Service{
		rt:         rt,
		store:      models.NewStore(rt),
		outbox:     &queue.Outbox{KeyBase: "chat", InstanceID: rt.Config.InstanceID},
		courier:    courier,
		senderStop: make(chan bool),
	}

	s.server = web.NewServer(rt, s)

	return s
}

func (s *Service) Start() error {
	log := slog.With("comp", "service")

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

func (s *Service) Store() models.Store { return s.store }

func (s *Service) StartChat(ctx context.Context, ch *models.Channel, chatID models.ChatID) (*models.Contact, bool, error) {
	log := slog.With("comp", "service")
	rc := s.rt.RP.Get()
	defer rc.Close()

	var contact *models.Contact
	var isNew bool
	var err error

	// if client provided a chat ID look for a matching contact
	if chatID != "" {
		contact, err = models.LoadContact(ctx, s.rt, ch.OrgID, chatID)
		if err != nil && err != sql.ErrNoRows {
			return nil, false, fmt.Errorf("error looking up contact: %w", err)
		}
	}

	// if not or if contact couldn't be found, generate a new random chat id, and have courier create a new contact
	if contact == nil {
		chatID = models.NewChatID()
		isNew = true

		if err := s.courier.StartChat(ch, chatID); err != nil {
			return nil, false, fmt.Errorf("error notifying courier of new chat: %w", err)
		}

		// contact should now exist now...
		contact, err = models.LoadContact(ctx, s.rt, ch.OrgID, chatID)
		if err != nil {
			return nil, false, fmt.Errorf("error looking up new contact: %w", err)
		}
	}

	// mark chat as ready to send messages
	if err := s.outbox.SetReady(rc, ch, chatID, true); err != nil {
		return nil, false, fmt.Errorf("error setting chat ready: %w", err)
	}

	log.Info("chat started", "chat_id", chatID)
	return contact, isNew, nil
}

func (s *Service) CreateMsgIn(ctx context.Context, ch *models.Channel, contact *models.Contact, text string) error {
	if err := s.courier.CreateMsg(ch, contact, text); err != nil {
		return fmt.Errorf("error notifying courier of new msg: %w", err)
	}
	return nil
}

func (s *Service) ConfirmMsgOut(ctx context.Context, ch *models.Channel, contact *models.Contact, msgID models.MsgID) error {
	rc := s.rt.RP.Get()
	defer rc.Close()

	// TODO send DLR to courier

	// mark chat as ready to send again
	if err := s.outbox.SetReady(rc, ch, contact.ChatID, true); err != nil {
		return fmt.Errorf("error setting chat ready: %w", err)
	}

	return nil
}

func (s *Service) CloseChat(ctx context.Context, ch *models.Channel, contact *models.Contact) error {
	log := slog.With("comp", "service")
	rc := s.rt.RP.Get()
	defer rc.Close()

	// mark chat as no longer ready
	if err := s.outbox.SetReady(rc, ch, contact.ChatID, false); err != nil {
		return fmt.Errorf("error unsetting chat ready: %w", err)
	}

	log.Info("chat closed", "chat_id", contact.ChatID)
	return nil
}

func (s *Service) QueueMsgOut(ctx context.Context, ch *models.Channel, contact *models.Contact, msg *models.MsgOut) error {
	rc := s.rt.RP.Get()
	defer rc.Close()

	if err := s.outbox.AddMessage(rc, ch, contact.ChatID, msg); err != nil {
		return fmt.Errorf("error queuing to outbox: %w", err)
	}

	return nil
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

	rc := s.rt.RP.Get()
	defer rc.Close()

	ready, err := s.outbox.ReadReady(rc)
	if err != nil {
		log.Error("error reading outboxes", "error", err)
		return
	}

	for outbox, msg := range ready {
		client := s.server.GetClient(outbox.ChatID)
		if client != nil {
			client.Send(events.NewChatMsgOut(msg))
		}
	}

	// TODO email or fail stale messages
}
