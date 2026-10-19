package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/chip/v26/utils"
)

// the most of a JSON request body we'll read - a visitor's message is bounded well below this by validation
const maxJSONBytes = 64 * 1024

func requestLogger(listener string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			next.ServeHTTP(ww, r)

			elapsed := time.Since(start)
			uri := fmt.Sprintf("https://%s%s", r.Host, r.URL.RequestURI())
			ww.Header().Set("X-Elapsed-NS", strconv.FormatInt(int64(elapsed), 10))

			// health checks are frequent and uninteresting
			if r.URL.Path != "/" {
				slog.Info("request completed", "listener", listener, "method", r.Method, "status", ww.Status(), "elapsed", elapsed, "length", ww.BytesWritten(), "url", uri, "user_agent", r.UserAgent())
			}
		})
	}
}

// recovers from panics, reports them and returns an HTTP 500 response
func panicRecovery(listener string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if panicVal := recover(); panicVal != nil {
					runtime.PanicHandler(panicVal, map[string]string{"listener": listener})

					writeError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// requireAuthToken refuses a request to the internal listener without the token our callers send
func (s *Server) requireAuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		// only checked if a token is configured (might not be for dev environments)
		if token := s.rt.Config.AuthToken; token != "" {
			if !strings.HasPrefix(auth, "Token ") || !utils.SecretEqual(auth[6:], token) {
				writeError(w, http.StatusUnauthorized, "invalid or missing authorization header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// readJSON reads the request's body as JSON into the given value and validates it. Both failing is the caller's
// fault and answered as a bad request.
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := utils.Validate(v); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}

// requestIP returns the requesting IP. The RealIP middleware has already resolved forwarded headers into
// RemoteAddr, which may or may not still carry a port.
func requestIP(r *http.Request) string {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}
