package tembachat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/nyaruka/gocommon/jsonx"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	config     *Config
	httpServer *http.Server
	wg         *sync.WaitGroup

	clients     map[string]*Client
	clientMutex *sync.RWMutex
}

func NewServer(cfg *Config) *Server {
	return &Server{
		config: cfg,
		httpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.Address, cfg.Port),
		},
		wg: &sync.WaitGroup{},

		clients:     make(map[string]*Client),
		clientMutex: &sync.RWMutex{},
	}
}

func (s *Server) Start() error {
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

func (s *Server) Stop() {
	log := slog.With("comp", "server")

	s.clientMutex.RLock()
	for _, c := range s.clients {
		c.Stop()
	}
	s.clientMutex.RUnlock()

	// shut down our HTTP server
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		log.Error("error shutting down http server", "error", err)
	} else {
		log.Info("http server stopped")
	}

	s.wg.Wait()
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
	log := slog.With("comp", "server")

	// hijack HTTP connection...
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("error upgrading connection", "error", err)
		return
	}

	client := NewClient(s, conn)
	s.register(client)

	client.Start()
}

type sendRequest struct {
	Client  string `json:"client" validate:"required"`
	Message string `json:"message" validate:"required"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	payload := &sendRequest{}

	if err := jsonx.UnmarshalWithLimit(r.Body, payload, 1024*1024); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	client := s.client(payload.Client)
	if client == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	client.Send(payload.Message)

	w.Write(jsonx.MustMarshal(map[string]any{"status": "queued"}))
}

func (s *Server) client(identifier string) *Client {
	defer s.clientMutex.RUnlock()

	s.clientMutex.RLock()
	return s.clients[identifier]
}

func (s *Server) register(c *Client) {
	s.clientMutex.Lock()
	s.clients[c.identifier] = c
	s.clientMutex.Unlock()

	slog.Info("client registered", "identifier", c.identifier)
}

func (s *Server) unregister(c *Client) {
	s.clientMutex.Lock()
	delete(s.clients, c.identifier)
	s.clientMutex.Unlock()

	slog.Info("client unregistered", "identifier", c.identifier)
}

func (s *Server) messageReceived(c *Client, m string) {
	// TODO call callback

	slog.Info("message received", "client", c.identifier, "message", string(m))
}
