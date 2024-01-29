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
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"golang.org/x/exp/maps"
)

type Service interface {
	Store() models.Store
	OnChatStarted(models.Channel, *models.Contact)
	OnChatReceive(models.Channel, *models.Contact, events.Event)
	OnSendRequest(*models.MsgOut)
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
	return &Server{
		rt: rt,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", rt.Config.Address, rt.Config.Port),
		},

		service: service,

		clients:     make(map[models.ChatID]*Client),
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

	channelUUID := models.ChannelUUID(r.URL.Query().Get("channel"))
	chatID := models.ChatID(r.URL.Query().Get("chat_id"))

	if !uuids.IsV4(string(channelUUID)) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid channel UUID")
		return
	}
	channel, err := s.service.Store().GetChannel(ctx, channelUUID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "no such channel")
		return
	}

	// if chat ID was provided, lookup the contact
	var contact *models.Contact
	isNew := false
	if chatID != "" {
		contact, err = models.LoadContact(ctx, s.rt, channel, chatID)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "error looking up contact")
			return
		}
	} else {
		contact = models.NewContact(channel)
		isNew = true
	}

	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 10)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "error upgrading connection")
		return
	}

	NewClient(s, sock, channel, contact, isNew)

	if isNew {
		s.service.OnChatStarted(channel, contact)
	}
}

type sendRequest struct {
	MsgID       models.MsgID       `json:"msg_id"       validate:"required"`
	ChannelUUID models.ChannelUUID `json:"channel_uuid" validate:"required"`
	ChatID      models.ChatID      `json:"chat_id"      validate:"required"`
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

	channel, err := s.service.Store().GetChannel(ctx, payload.ChannelUUID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "channel not found")
		return
	}

	var user models.User
	if payload.UserID != models.NilUserID {
		user, err = s.service.Store().GetUser(ctx, payload.UserID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
	}

	s.service.OnSendRequest(models.NewMsgOut(payload.MsgID, channel, payload.ChatID, payload.Text, payload.Origin, user))

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *Server) GetClient(chatID models.ChatID) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[chatID]
}

func (s *Server) Connect(c *Client) {
	s.clientMutex.Lock()
	s.clients[c.contact.ChatID] = c
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Add(1)

	slog.Info("client connected", "chat_id", c.contact.ChatID, "total", total)
}

func (s *Server) Disconnect(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.contact.ChatID)
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client disconnected", "chat_id", c.contact.ChatID, "total", total)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
