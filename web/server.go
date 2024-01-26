package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"golang.org/x/exp/maps"
)

type Server struct {
	rt         *runtime.Runtime
	store      models.Store
	httpServer *http.Server
	wg         sync.WaitGroup

	onSendRequest func(*models.MsgOut)
	onChatReceive func(*Client, events.Event)

	clients     map[string]*Client
	clientMutex *sync.RWMutex
}

func NewServer(rt *runtime.Runtime, store models.Store, onSendRequest func(*models.MsgOut), onChatReceive func(*Client, events.Event)) *Server {
	return &Server{
		rt:    rt,
		store: store,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", rt.Config.Address, rt.Config.Port),
		},

		onSendRequest: onSendRequest,
		onChatReceive: onChatReceive,

		clients:     make(map[string]*Client),
		clientMutex: &sync.RWMutex{},
	}
}

func (s *Server) Start() {
	log := slog.With("comp", "webserver", "address", s.rt.Config.Address, "port", s.rt.Config.Port)

	http.HandleFunc("/start", s.handleStart)
	http.HandleFunc("/send", s.handleSend)

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

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channelUUID := r.URL.Query().Get("channel")
	if !uuids.IsV4(channelUUID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid channel UUID")
		return
	}
	channel, err := s.store.GetChannel(ctx, models.ChannelUUID(channelUUID))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "no such channel")
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	email := r.URL.Query().Get("email")

	if chatID != "" {
		// convert chatID and email to a webchat URN amd check that's valid
		urn := models.NewURN(chatID, email)
		if err := urn.Validate(); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid chat ID or email")
			return
		}

		// and that it actually exists
		exists, err := models.URNExists(ctx, s.rt, channel, urn)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "error looking up URN")
			return
		}
		if !exists {
			writeErrorResponse(w, http.StatusBadRequest, "no such chat ID or email")
			return
		}
	}

	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 10)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "error upgrading connection")
		return
	}

	NewClient(s, sock, channel, chatID, email)
}

type sendRequest struct {
	MsgID       models.MsgID       `json:"msg_id"       validate:"required"`
	ChannelUUID models.ChannelUUID `json:"channel_uuid" validate:"required"`
	URN         urns.URN           `json:"urn"          validate:"required"`
	Text        string             `json:"text"         validate:"required"`
	Origin      models.MsgOrigin   `json:"origin"       validate:"required"`
	UserID      models.UserID      `json:"user_id"`
}

// handles a send message request from courier
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	payload := &sendRequest{}

	if err := jsonx.UnmarshalWithLimit(r.Body, payload, 1024*1024); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "error reading request")
		return
	}
	channel, err := s.store.GetChannel(ctx, payload.ChannelUUID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "channel not found")
		return
	}

	if payload.URN.Scheme() != urns.WebChatScheme || payload.URN.Validate() != nil {
		writeErrorResponse(w, http.StatusBadRequest, "URN is not valid webchat URN")
		return
	}
	chatID, email := models.ParseURN(payload.URN)

	var user models.User
	if payload.UserID != models.NilUserID {
		user, err = s.store.GetUser(ctx, payload.UserID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
	}

	s.onSendRequest(models.NewMsgOut(payload.MsgID, channel, chatID, email, payload.Origin, user))

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *Server) GetClient(identifier string) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[identifier]
}

func (s *Server) Connect(c *Client) {
	s.clientMutex.Lock()
	s.clients[c.ChatID()] = c
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Add(1)

	slog.Info("client connected", "chat_id", c.ChatID(), "email", c.Email(), "total", total)
}

func (s *Server) Disconnect(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.ChatID())
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client disconnected", "chat_id", c.ChatID(), "email", c.Email(), "total", total)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
