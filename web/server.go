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
	"github.com/nyaruka/tembachat/courier"
	"github.com/nyaruka/tembachat/runtime"
	"golang.org/x/exp/maps"
)

type Server struct {
	rt         *runtime.Runtime
	store      models.Store
	httpServer *http.Server
	wg         sync.WaitGroup

	clients     map[string]*Client
	clientMutex *sync.RWMutex
}

func NewServer(rt *runtime.Runtime, store models.Store) *Server {
	return &Server{
		rt:    rt,
		store: store,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", rt.Config.Address, rt.Config.Port),
		},
		clients:     make(map[string]*Client),
		clientMutex: &sync.RWMutex{},
	}
}

func (s *Server) Start() {
	log := slog.With("comp", "webserver", "address", s.rt.Config.Address, "port", s.rt.Config.Port)

	http.HandleFunc("/", s.handleIndex)
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

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.clientMutex.RLock()
	resp := make(map[string]any, len(s.clients))
	for id := range s.clients {
		resp[id] = true
	}
	s.clientMutex.RUnlock()

	w.Write(jsonx.MustMarshal(resp))
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

	identifier := r.URL.Query().Get("identifier")
	if identifier != "" {
		// if we're resuming from an existing identifier, check that it's valid...
		urn, err := urns.NewWebChatURN(identifier)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid client identifier")
			return
		}

		// and that it actually exists
		exists, err := models.URNExists(ctx, s.rt, channel, urn)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "error checking identifier")
			return
		}
		if !exists {
			writeErrorResponse(w, http.StatusBadRequest, "invalid client identifier")
			return
		}
	}

	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 10)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "error upgrading connection")
		return
	}

	NewClient(s, sock, channel, identifier)
}

type sendRequest struct {
	Identifier string        `json:"identifier" validate:"required"`
	Text       string        `json:"text" validate:"required"`
	Origin     string        `json:"origin" validate:"required"`
	UserID     models.UserID `json:"user_id"`
}

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

	client := s.client(payload.Identifier)
	if client == nil {
		writeErrorResponse(w, http.StatusNotFound, "no such client")
		return
	}

	var user models.User
	var err error
	if payload.UserID != models.NilUserID {
		user, err = s.store.GetUser(ctx, payload.UserID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "no such user")
			return
		}
	}

	client.Send(events.NewMsgOut(payload.Text, payload.Origin, user))

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *Server) client(identifier string) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[identifier]
}

func (s *Server) Connect(c *Client) {
	s.clientMutex.Lock()
	s.clients[c.Identifier()] = c
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Add(1)

	slog.Info("client connected", "identifier", c.Identifier(), "total", total)
}

func (s *Server) Disconnect(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.Identifier())
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client disconnected", "identifier", c.Identifier(), "total", total)
}

func (s *Server) NotifyCourier(c *Client, e events.Event) {
	courier.Notify(s.rt.Config, c.Channel(), c.identifier, e)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
