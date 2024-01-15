package tembachat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/courier"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
	"golang.org/x/exp/maps"
)

type server struct {
	config     *runtime.Config
	httpServer *http.Server
	wg         sync.WaitGroup

	clients     map[string]webchat.Client
	clientMutex *sync.RWMutex
}

func NewServer(cfg *runtime.Config) webchat.Server {
	return &server{
		config: cfg,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.Address, cfg.Port),
		},

		clients:     make(map[string]webchat.Client),
		clientMutex: &sync.RWMutex{},
	}
}

func (s *server) Start() error {
	log := slog.With("comp", "server", "address", s.config.Address, "port", s.config.Port)

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

	s.wg.Wait()
}

func (s *server) Config() *runtime.Config { return s.config }

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
	channelUUID := r.URL.Query().Get("channel")
	if !uuids.IsV4(channelUUID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid channel UUID")
		return
	}

	identifier := r.URL.Query().Get("identifier")
	if identifier != "" {
		_, err := urns.NewWebChatURN(identifier)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid client identifier")
			slog.Error("invalid client identifier", "error", err)
			return
		}
	}

	// hijack the HTTP connection...
	sock, err := httpx.NewWebSocket(w, r, 4096, 10)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "error upgrading connection")
		return
	}

	webchat.NewClient(s, sock, webchat.NewChannel(uuids.UUID(channelUUID)), identifier)
}

type sendRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Text       string `json:"text" validate:"required"`
	Origin     string `json:"origin" validate:"required"`
}

func (s *server) handleSend(w http.ResponseWriter, r *http.Request) {
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

	client.Send(webchat.NewMsgOutEvent(payload.Text, payload.Origin))

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *server) client(identifier string) webchat.Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[identifier]
}

func (s *server) Register(c webchat.Client) {
	s.clientMutex.Lock()
	s.clients[c.Identifier()] = c
	s.clientMutex.Unlock()

	s.wg.Add(1)

	slog.Info("client registered", "identifier", c.Identifier())
}

func (s *server) Unregister(c webchat.Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.Identifier())
	s.clientMutex.Unlock()

	s.wg.Done()

	slog.Info("client unregistered", "identifier", c.Identifier())
}

func (s *server) NotifyCourier(c webchat.Client, e webchat.Event) {
	courier.Notify(s.config, c, e)
}

func writeErrorResponse(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(map[string]string{"error": msg}))
}
