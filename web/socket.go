package web

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/gocommon/dates"
)

// The realtime server fronts the browser's WebSocket and calls these proxies, server-to-server, to have us
// authorize each subscription a widget asks for: subscribe when it first asks, and sub_refresh periodically after
// that, at the expiry we hand back. Visitors connect anonymously - no session, no user - so authorization is
// capability-based: the socket name embeds a secret chat-id, so possession of the name is the credential, and a
// visitor can subscribe to (only) their own chat.
//
// Both proxies also record the subscription in valkey, as presence: a per-socket key with a TTL, re-armed on every
// subscribe and refresh, that says the socket currently has a subscriber. There's no unsubscribe callback, so the
// TTL is the only garbage collection, and whatever publishes to a socket checks the key first so as not to send to
// nobody. The key name and its presence-via-existence semantics are the centrifugo package's contract.

const (
	// how far ahead we set a subscription's expiry, scheduling the realtime server to call sub_refresh before it
	// lapses so we can re-authorize
	subscriptionWindow = 60 * time.Second

	// how long a socket's presence key survives without a refresh. It must comfortably exceed the window plus
	// the realtime server's refresh delay: the server drives sub_refresh from the expiry we return (one window),
	// but refresh requests can be delayed up to ~1 minute, so consecutive refreshes can be ~window + 60s apart.
	// 150s keeps the key alive across that gap while still expiring within a couple of minutes once the last
	// subscriber stops refreshing.
	subscriptionTTL = 150 * time.Second
)

// the chat sockets we authorize - chat:<channel-uuid>:<chat-id> - with the pattern doing all the shape validation:
// a socket that doesn't fully match (wrong namespace or number of segments, a non-canonical UUID, a chat-id that's
// not the right length or alphabet) is denied before anything is looked up. Only canonical lowercase UUIDs are
// accepted because a socket name is an exact string key: events are published to the canonical form, so a
// subscription to any other encoding could never receive anything anyway.
var chatSocketRegex = regexp.MustCompile(`^chat:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}):([a-zA-Z0-9]{24})$`)

// the realtime server's own term for a socket is channel, which is already taken by messaging channels - this is
// its protocol, so it keeps its naming
type proxyRequest struct {
	Channel string `json:"channel"`
}

type proxyResponse struct {
	Result *proxyResult `json:"result,omitempty"`
	Error  *proxyError  `json:"error,omitempty"`
}

type proxyResult struct {
	ExpireAt int64 `json:"expire_at,omitempty"`
	Expired  bool  `json:"expired,omitempty"`
}

type proxyError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handleSubscribe is the subscribe proxy, called when a browser asks to subscribe to a socket. If it may, we record
// the subscription and return the expiry at which the realtime server should re-check it via sub_refresh. Anything
// else is refused with a forbidden error, which the realtime server surfaces to the browser as a failed subscribe
// without tearing down the whole connection. Denials are returned in-body with HTTP 200, as the proxy protocol
// requires.
func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	socket, allowed, err := s.authorizeSocket(r)
	if err != nil {
		logRequestError(r, "error authorizing subscription", err)
		writeError(w, http.StatusInternalServerError, "error authorizing subscription")
		return
	}
	if !allowed {
		writeJSON(w, http.StatusOK, &proxyResponse{Error: &proxyError{Code: 403, Message: "forbidden"}})
		return
	}
	if err := s.recordSubscription(r.Context(), socket); err != nil {
		logRequestError(r, "error recording subscription", err)
		writeError(w, http.StatusInternalServerError, "error recording subscription")
		return
	}

	writeJSON(w, http.StatusOK, &proxyResponse{Result: &proxyResult{ExpireAt: dates.Now().Add(subscriptionWindow).Unix()}})
}

// handleSubRefresh is the sub_refresh proxy, called periodically before a subscription's expiry. We re-run the same
// authorization as subscribe (the chat may have gone away since), and either re-arm the presence key and return a
// fresh expiry or let the subscription expire.
func (s *Server) handleSubRefresh(w http.ResponseWriter, r *http.Request) {
	socket, allowed, err := s.authorizeSocket(r)
	if err != nil {
		logRequestError(r, "error authorizing subscription", err)
		writeError(w, http.StatusInternalServerError, "error authorizing subscription")
		return
	}
	if !allowed {
		writeJSON(w, http.StatusOK, &proxyResponse{Result: &proxyResult{Expired: true}})
		return
	}
	if err := s.recordSubscription(r.Context(), socket); err != nil {
		logRequestError(r, "error recording subscription", err)
		writeError(w, http.StatusInternalServerError, "error recording subscription")
		return
	}

	writeJSON(w, http.StatusOK, &proxyResponse{Result: &proxyResult{ExpireAt: dates.Now().Add(subscriptionWindow).Unix()}})
}

// authorizeSocket reads the socket a proxy request is about and decides whether its subscription is allowed: the
// socket must be a chat socket of an active webchat channel, with a chat on it under that chat-id, i.e. a webchat
// URN with that chat-id on that channel for an active contact. A URN always shares its channel's workspace, so
// this also scopes the chat to the channel's own workspace, and requiring an active contact means releasing a
// contact closes their chat immediately.
//
// On top of that credential, a channel configured with allowed_domains is pinned to its operator's own websites:
// if the request carries a browser Origin header (forwarded from the WebSocket handshake by the realtime server),
// its host[:port] must match one of the configured entries - the same restriction the channel's HTTP endpoints
// apply. An absent header (a non-browser client, or a proxy config that doesn't forward it) or an empty config
// allows as before.
func (s *Server) authorizeSocket(r *http.Request) (string, bool, error) {
	ctx := r.Context()

	req := &proxyRequest{}
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBytes)).Decode(req); err != nil {
		return "", false, nil // a malformed request is just a denial, not a failure
	}

	match := chatSocketRegex.FindStringSubmatch(req.Channel)
	if match == nil {
		return req.Channel, false, nil
	}
	channelUUID, chatID := models.ChannelUUID(match[1]), match[2]

	ch, err := models.LoadChannel(ctx, s.rt.DB, channelUUID)
	if err != nil {
		return req.Channel, false, err
	}
	if ch == nil {
		return req.Channel, false, nil
	}

	if domains := ch.AllowedDomains(); len(domains) > 0 {
		if origin := r.Header.Get("Origin"); origin != "" && !originAllowed(origin, domains) {
			return req.Channel, false, nil
		}
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, chatID)
	if err != nil {
		return req.Channel, false, err
	}

	return req.Channel, visitor != nil, nil
}

// recordSubscription marks a socket as having at least one active subscriber by (re)setting its presence key
func (s *Server) recordSubscription(ctx context.Context, socket string) error {
	vc := s.rt.VK.Get()
	defer vc.Close()

	_, err := vc.Do("SET", centrifugo.SubscriptionKey(socket), "1", "EX", int(subscriptionTTL.Seconds()))
	return err
}
