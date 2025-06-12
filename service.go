package chip

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/core/courier"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/core/queue"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/chip/web"
	"github.com/nyaruka/chip/web/events"
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

func NewService(rt *runtime.Runtime, courier courier.Courier) *Service {
	s := &Service{
		rt:         rt,
		store:      models.NewStore(rt),
		outboxes:   &queue.Outboxes{KeyBase: "chat", InstanceID: rt.Config.InstanceID},
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

		if err := s.courier.StartChat(ctx, ch, chatID); err != nil {
			return nil, false, fmt.Errorf("error notifying courier of new chat: %w", err)
		}

		// contact should now exist now...
		contact, err = models.LoadContact(ctx, s.rt, ch.OrgID, chatID)
		if err != nil {
			return nil, false, fmt.Errorf("error looking up new contact: %w", err)
		}
	}

	// mark chat as ready to send messages (non-fatal if Redis is unavailable)
	s.rt.WithRedisConn(func(rc redis.Conn) error {
		return s.outboxes.SetReady(rc, ch, chatID, true)
	})

	log.Info("chat started", "chat_id", chatID)
	return contact, isNew, nil
}

func (s *Service) CreateMsgIn(ctx context.Context, ch *models.Channel, contact *models.Contact, text string) error {
	if err := s.courier.CreateMsg(ctx, ch, contact, text); err != nil {
		return fmt.Errorf("error notifying courier of new msg: %w", err)
	}
	return nil
}

func (s *Service) ConfirmDelivery(ctx context.Context, ch *models.Channel, contact *models.Contact, itemID queue.ItemID) error {
	// if this is a message, tell courier it was delivered
	if strings.HasPrefix(string(itemID), "m") {
		msgID, err := strconv.Atoi(strings.TrimPrefix(string(itemID), "m"))
		if err != nil {
			return fmt.Errorf("error parsing msg id: %w", err)
		}

		if err := s.courier.ReportDelivered(ctx, ch, contact, models.MsgID(msgID)); err != nil {
			return fmt.Errorf("error notifying courier of delivery: %w", err)
		}
	}

	// record sent status in Redis (non-fatal if Redis is unavailable)
	s.rt.WithRedisConn(func(rc redis.Conn) error {
		_, err := s.outboxes.RecordSent(rc, ch, contact.ChatID, itemID)
		return err
	})

	return nil
}

func (s *Service) CloseChat(ctx context.Context, ch *models.Channel, contact *models.Contact) error {
	log := slog.With("comp", "service")

	// mark chat as no longer ready (non-fatal if Redis is unavailable)
	s.rt.WithRedisConn(func(rc redis.Conn) error {
		return s.outboxes.SetReady(rc, ch, contact.ChatID, false)
	})

	log.Info("chat closed", "chat_id", contact.ChatID)
	return nil
}

func (s *Service) QueueMsgOut(ctx context.Context, ch *models.Channel, contact *models.Contact, msg *models.MsgOut) error {
	// queue message to outbox (non-fatal if Redis is unavailable)
	s.rt.WithRedisConn(func(rc redis.Conn) error {
		return s.outboxes.AddMessage(rc, ch, contact.ChatID, msg)
	})

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

	if s.rt.RP == nil {
		// Redis unavailable, skip sending
		return
	}

	rc := s.rt.RP.Get()
	defer rc.Close()

	ready, err := s.outboxes.ReadReady(rc)
	if err != nil {
		log.Error("error reading outboxes", "error", err)
		return
	}

	for outbox, item := range ready {
		client := s.server.GetClient(outbox.ChatID)
		if client != nil {
			client.Send(events.NewChatMsgOut(item.Msg))
		}
	}

	// TODO email or fail stale messages
}
