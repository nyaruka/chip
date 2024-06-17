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
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"golang.org/x/exp/maps"
)

type Service interface {
	Store() models.Store
	OnChatStarted(*models.Channel, models.ChatID) error
	OnChatMsgIn(*models.Channel, *models.Contact, string) error
	OnChatClosed(*models.Channel, *models.Contact) error
	OnSendRequest(*models.Channel, *models.MsgOut) error
}

type Server struct {
	rt         *runtime.Runtime
	service    Service
	httpServer *http.Server
	wg         sync.WaitGroup

	clients     map[string]*Client
	clientMutex *sync.RWMutex
}

func NewServer(rt *runtime.Runtime, service Service) *Server {
	s := &Server{
		rt:      rt,
		service: service,

		clients:     make(map[string]*Client),
		clientMutex: &sync.RWMutex{},
	}

	router := chi.NewRouter()
	router.Use(middleware.Compress(flate.DefaultCompression))
	router.Use(middleware.StripSlashes)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Get("/", s.handleIndex)
	router.Handle("/wc/connect/{channel:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", s.channelHandler(s.handleConnect))
	router.Handle("/wc/send/{channel:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", s.channelHandler(s.handleSend))

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", rt.Config.Address, rt.Config.Port),
		Handler: router,
	}

	return s
}

func (s *Server) Start() {
	log := s.log().With("address", s.rt.Config.Address, "port", s.rt.Config.Port)

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
	s.log().Info("stopping...")

	s.clientMutex.RLock()
	clients := maps.Values(s.clients)
	s.clientMutex.RUnlock()

	for _, c := range clients {
		c.Stop()
	}

	// shut down our HTTP server
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		s.log().Error("error shutting down http server", "error", err)
	}

	s.wg.Wait()

	s.log().Info("stopped")
}

func (s *Server) channelHandler(fn func(context.Context, *http.Request, http.ResponseWriter, *models.Channel)) http.HandlerFunc {
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

func (s *Server) handleConnect(ctx context.Context, r *http.Request, w http.ResponseWriter, ch *models.Channel) {
	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 0)
	if err != nil {
		s.log().Error("error hijacking connection", "error", err)
		return
	}

	client := NewClient(s, sock, ch)

	s.clientMutex.Lock()
	s.clients[client.id] = client
	total := len(s.clients)
	s.clientMutex.Unlock()
	s.wg.Add(1)

	s.log().Info("client connected", "channel", ch.UUID, "client_id", client.id, "total", total)
}

type sendRequest struct {
	ChatID models.ChatID `json:"chat_id"              validate:"required"`
	Secret string        `json:"secret"               validate:"required"`
	Msg    struct {
		ID          models.MsgID     `json:"id"       validate:"required"`
		Text        string           `json:"text"`
		Attachments []string         `json:"attachments"`
		Origin      models.MsgOrigin `json:"origin"   validate:"required"`
		UserID      models.UserID    `json:"user_id"`
	} `json:"msg"`
}

// handles a send message request from courier
func (s *Server) handleSend(ctx context.Context, r *http.Request, w http.ResponseWriter, ch *models.Channel) {
	payload := &sendRequest{}
	if err := jsonx.UnmarshalWithLimit(r.Body, payload, 1024*1024); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading request: %s", err))
		return
	}

	if ch.Secret() != payload.Secret {
		writeErrorResponse(w, http.StatusBadRequest, "channel secret incorrect")
		return
	}

	var user *models.User
	var err error
	if payload.Msg.UserID != models.NilUserID {
		user, err = s.service.Store().GetUser(ctx, payload.Msg.UserID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
	}

	err = s.service.OnSendRequest(ch, models.NewMsgOut(payload.Msg.ID, ch, payload.ChatID, payload.Msg.Text, payload.Msg.Attachments, payload.Msg.Origin, user, time.Now()))
	if err == nil {
		w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
	} else {
		s.log().Error("error handing send request", "error", err)

		writeErrorResponse(w, http.StatusInternalServerError, "unable to queue message")
		return
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write(jsonx.MustMarshal(map[string]string{"version": s.rt.Config.Version}))
}

func (s *Server) GetClient(chatID models.ChatID) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()

	// TODO maintain map by contact ?
	for _, c := range s.clients {
		if c.chatID() == chatID {
			return c
		}
	}
	return nil
}

func (s *Server) OnDisconnect(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.id)
	total := len(s.clients)
	s.clientMutex.Unlock()
	s.wg.Done()

	s.log().Info("client disconnected", "total", total)
}

func (s *Server) log() *slog.Logger {
	return slog.With("comp", "server")
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
