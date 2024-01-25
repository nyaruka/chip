package tembachat

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
	"github.com/nyaruka/redisx"
	"github.com/nyaruka/tembachat/courier"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
	"github.com/nyaruka/tembachat/webchat/events"
	"github.com/nyaruka/tembachat/webchat/models"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

type server struct {
	rt         *runtime.Runtime
	httpServer *http.Server
	store      Store
	wg         sync.WaitGroup

	clients     map[string]webchat.Client
	clientMutex *sync.RWMutex
}

func NewServer(cfg *runtime.Config) webchat.Server {
	rt := &runtime.Runtime{Config: cfg}
	return &server{
		rt: rt,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.Address, cfg.Port),
		},
		store: NewStore(rt),

		clients:     make(map[string]webchat.Client),
		clientMutex: &sync.RWMutex{},
	}
}

func (s *server) Start() error {
	log := slog.With("comp", "server", "address", s.rt.Config.Address, "port", s.rt.Config.Port)
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

	log.Info("server started")
	return nil
}

func (s *server) Stop() {
	log := slog.With("comp", "server")

	log.Info("stopping server...")

	s.clientMutex.RLock()
	clients := maps.Values(s.clients)
	s.clientMutex.RUnlock()

	for _, c := range clients {
		c.Stop()
	}

	// shut down our HTTP server
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		log.Error("error shutting down http server", "error", err)
	} else {
		log.Info("http server stopped")
	}

	s.store.Close()

	s.wg.Wait()
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.clientMutex.RLock()
	resp := make(map[string]any, len(s.clients))
	for id := range s.clients {
		resp[id] = true
	}
	s.clientMutex.RUnlock()

	w.Write(jsonx.MustMarshal(resp))
}

func (s *server) handleStart(w http.ResponseWriter, r *http.Request) {
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

	webchat.NewClient(s, sock, channel, identifier)
}

type sendRequest struct {
	Identifier string        `json:"identifier" validate:"required"`
	Text       string        `json:"text" validate:"required"`
	Origin     string        `json:"origin" validate:"required"`
	UserID     models.UserID `json:"user_id"`
}

func (s *server) handleSend(w http.ResponseWriter, r *http.Request) {
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

func (s *server) client(identifier string) webchat.Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[identifier]
}

func (s *server) Connect(c webchat.Client) {
	s.clientMutex.Lock()
	s.clients[c.Identifier()] = c
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Add(1)

	slog.Info("client connected", "identifier", c.Identifier(), "total", total)
}

func (s *server) Disconnect(c webchat.Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.Identifier())
	total := len(s.clients)
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client disconnected", "identifier", c.Identifier(), "total", total)
}

func (s *server) NotifyCourier(c webchat.Client, e events.Event) {
	courier.Notify(s.rt.Config, c, e)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
