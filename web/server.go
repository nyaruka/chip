package web

import (
	"compress/flate"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"golang.org/x/exp/maps"
)

type Service interface {
	Store() models.Store
	OnChatStarted(models.Channel, *models.Contact)
	OnChatReceive(models.Channel, *models.Contact, events.Event)
	OnSendRequest(models.Channel, *models.MsgOut)
}

type Server struct {
	rt         *runtime.Runtime
	service    Service
	httpServer *http.Server
	wg         sync.WaitGroup

	clients     map[models.ChatID]*Client
	clientMutex *sync.RWMutex
}

func NewServer(rt *runtime.Runtime, service Service) *Server {
	s := &Server{
		rt:      rt,
		service: service,

		clients:     make(map[models.ChatID]*Client),
		clientMutex: &sync.RWMutex{},
	}

	router := chi.NewRouter()
	router.Use(middleware.Compress(flate.DefaultCompression))
	router.Use(middleware.StripSlashes)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Post("/start/{channel:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", s.channelHandler(s.handleStart))
	router.Post("/send/{channel:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", s.channelHandler(s.handleSend))

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", rt.Config.Address, rt.Config.Port),
		Handler: router,
	}

	return s
}

func (s *Server) Start() {
	log := slog.With("comp", "webserver", "address", s.rt.Config.Address, "port", s.rt.Config.Port)

	s.wg.Add(1)

	go func() {
		defer s.wg.Done()

		err := s.httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error("error listening", "error", err)
		}
	}()

	log.Info("started")
}

func (s *Server) Stop() {
	log := slog.With("comp", "server")

	log.Info("stopping...")

	s.clientMutex.RLock()
	clients := maps.Values(s.clients)
	s.clientMutex.RUnlock()

	for _, c := range clients {
		c.Stop()
	}

	// shut down our HTTP server
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		log.Error("error shutting down http server", "error", err)
	}

	s.wg.Wait()

	log.Info("stopped")
}

func (s *Server) channelHandler(fn func(context.Context, *http.Request, http.ResponseWriter, models.Channel)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		channelUUID := models.ChannelUUID(r.PathValue("channel"))

		ch, err := s.service.Store().GetChannel(r.Context(), channelUUID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "no such channel")
			return
		}

		fn(r.Context(), r, w, ch)
	}
}

func (s *Server) handleStart(ctx context.Context, r *http.Request, w http.ResponseWriter, ch models.Channel) {
	chatID := models.ChatID(r.URL.Query().Get("chat_id"))

	// if chat ID was provided, lookup the contact
	var contact *models.Contact
	var err error
	isNew := false
	if chatID != "" {
		contact, err = models.LoadContact(ctx, s.rt, ch, chatID)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "error looking up contact")
			return
		}
	} else {
		contact = models.NewContact(ch)
		isNew = true
	}

	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 0)
	if err != nil {
		return
	}

	s.clientMutex.Lock()

	if _, exists := s.clients[contact.ChatID]; !exists {
		s.clients[contact.ChatID] = NewClient(s, sock, ch, contact, isNew)
	} else {
		sock.Close(1008)

		slog.Info("rejected duplicate connection", "channel", ch.UUID(), "chat_id", contact.ChatID)
	}

	total := len(s.clients)
	s.clientMutex.Unlock()
	s.wg.Add(1)
	slog.Info("client connected", "channel", ch.UUID(), "chat_id", contact.ChatID, "total", total)

	if isNew {
		s.service.OnChatStarted(ch, contact)
	}
}

type sendRequest struct {
	ChatID models.ChatID `json:"chat_id"         validate:"required"`
	Secret string        `json:"secret"          validate:"required"`
	Msg    struct {
		ID     models.MsgID     `json:"id"       validate:"required"`
		Text   string           `json:"text"     validate:"required"`
		Origin models.MsgOrigin `json:"origin"   validate:"required"`
		UserID models.UserID    `json:"user_id"`
	} `json:"msg"`
}

// handles a send message request from courier
func (s *Server) handleSend(ctx context.Context, r *http.Request, w http.ResponseWriter, ch models.Channel) {
	payload := &sendRequest{}
	if err := jsonx.UnmarshalWithLimit(r.Body, payload, 1024*1024); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "error reading request")
		return
	}

	if ch.Secret() != payload.Secret {
		writeErrorResponse(w, http.StatusBadRequest, "channel secret incorrect")
		return
	}

	var user models.User
	var err error
	if payload.Msg.UserID != models.NilUserID {
		user, err = s.service.Store().GetUser(ctx, payload.Msg.UserID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
	}

	s.service.OnSendRequest(ch, models.NewMsgOut(payload.Msg.ID, payload.ChatID, payload.Msg.Text, payload.Msg.Origin, user, time.Now()))

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *Server) GetClient(chatID models.ChatID) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[chatID]
}

func (s *Server) OnDisconnect(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.contact.ChatID)
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client disconnected", "channel", c.channel.UUID(), "chat_id", c.contact.ChatID, "total", total)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
