// Package web is chip's HTTP surface: the public listener the widget talks to, and the internal one that courier
// and the realtime server talk to.
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nyaruka/chip/v26/core/courier"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/jsonx"
)

const (
	// the prefix of the public listener's routes, under which each channel's endpoints are addressed by its UUID
	publicPrefix = "/wc"

	// the prefix of the internal listener's routes, as the internal load balancer routes them
	internalPrefix = "/wi"
)

// Server serves on two listeners: a public one for the chat widget, whose requests are each addressed to a webchat
// channel, and an internal one for courier's sends and the realtime server's subscription proxies.
type Server struct {
	rt      *runtime.Runtime
	courier courier.Client

	publicServer   *http.Server
	internalServer *http.Server
	publicLn       net.Listener
	internalLn     net.Listener

	wg sync.WaitGroup
}

// NewServer creates a new server for the given runtime, talking to courier through the given client. The server
// will have to be started afterwards.
func NewServer(rt *runtime.Runtime, courier courier.Client) *Server {
	s := &Server{rt: rt, courier: courier}

	// public: the widget's endpoints, which it calls from whatever site it's embedded on
	public := chi.NewRouter()
	public.Use(middleware.RequestID)
	public.Use(middleware.RealIP)
	public.Use(panicRecovery("public"))
	public.Use(middleware.Timeout(30 * time.Second))
	public.Use(requestLogger("public"))
	public.Get("/", s.handleHealth("public"))
	public.Route(publicPrefix+"/{uuid}", func(r chi.Router) {
		r.Post("/start", s.channelRoute(s.handleStart))
		r.Post("/receive", s.channelRoute(s.handleReceive))
		r.Post("/status", s.channelRoute(s.handleStatus))
		r.Get("/history", s.channelRoute(s.handleHistory))
		r.Post("/upload", s.channelRoute(s.handleUpload))

		// the widget runs on third-party websites, so every endpoint needs CORS preflight support
		r.Options("/{action}", s.handlePreflight)
	})

	// internal: courier's sends and the realtime server's proxies, under the prefix the internal load balancer
	// routes to us
	internal := chi.NewRouter()
	internal.Use(middleware.RequestID)
	internal.Use(panicRecovery("internal"))
	internal.Use(middleware.Timeout(30 * time.Second))
	internal.Use(requestLogger("internal"))
	internal.Get("/", s.handleHealth("internal"))
	internal.Group(func(r chi.Router) {
		r.Use(s.requireAuthToken)
		r.Post(internalPrefix+"/send", s.handleSend)
		r.Post(internalPrefix+"/event", s.handleEvent)
		r.Post(internalPrefix+"/socket/subscribe", s.handleSubscribe)
		r.Post(internalPrefix+"/socket/sub_refresh", s.handleSubRefresh)
	})

	cfg := rt.Config

	s.publicServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Address, cfg.Port),
		Handler:      public,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  90 * time.Second,
	}
	s.internalServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.InternalAddress, cfg.InternalPort),
		Handler:      internal,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	return s
}

// PublicHandler returns the public listener's handler, for tests to make requests of without listeners
func (s *Server) PublicHandler() http.Handler {
	return s.publicServer.Handler
}

// InternalHandler returns the internal listener's handler, for tests
func (s *Server) InternalHandler() http.Handler {
	return s.internalServer.Handler
}

// handleHealth returns the liveness probe used by load balancer health checks
func (s *Server) handleHealth(listener string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"component": "chip",
			"listener":  listener,
			"version":   s.rt.Config.Version,
		})
	}
}

// writeJSON answers with the given value as JSON
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonx.MustMarshal(v))
}

// writeError answers with an error message as JSON
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// logRequestError logs an error from serving a request, unless it's only that the client went away mid-request,
// which cancels the request's context and so fails whatever it was doing - that's not ours to fix. We go by the
// context rather than the error because a query canceled while running fails with the database's own error.
func logRequestError(r *http.Request, msg string, err error, args ...any) {
	args = append([]any{"comp", "server", "error", err, "url", r.URL.String()}, args...)

	if errors.Is(r.Context().Err(), context.Canceled) {
		slog.Debug(msg, args...)
	} else {
		slog.Error(msg, args...)
	}
}

// Start binds the listeners and starts serving on them. It returns an error if a listener can't be bound, so that
// a failure is known before anything else starts.
func (s *Server) Start() error {
	publicLn, err := net.Listen("tcp", s.publicServer.Addr)
	if err != nil {
		return fmt.Errorf("error binding public listener on %s: %w", s.publicServer.Addr, err)
	}
	internalLn, err := net.Listen("tcp", s.internalServer.Addr)
	if err != nil {
		publicLn.Close()
		return fmt.Errorf("error binding internal listener on %s: %w", s.internalServer.Addr, err)
	}

	s.publicLn, s.internalLn = publicLn, internalLn

	s.serve("public", s.publicServer, publicLn)
	s.serve("internal", s.internalServer, internalLn)

	return nil
}

// Addrs returns the addresses the public and internal listeners are bound to, once started
func (s *Server) Addrs() (public, internal string) {
	return s.publicLn.Addr().String(), s.internalLn.Addr().String()
}

func (s *Server) serve(listener string, server *http.Server, ln net.Listener) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		log := slog.With("comp", "server", "listener", listener, "address", server.Addr)
		log.Info("server started", "version", s.rt.Config.Version)

		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("error listening", "error", err)
		}
	}()
}

// Stop stops the server, returning only after both listeners have stopped
func (s *Server) Stop() error {
	log := slog.With("comp", "server")
	log.Info("stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for listener, server := range map[string]*http.Server{"public": s.publicServer, "internal": s.internalServer} {
		if err := server.Shutdown(ctx); err != nil {
			log.Error("error shutting down server", "listener", listener, "error", err)
		}
	}

	s.wg.Wait()

	log.Info("server stopped")
	return nil
}
