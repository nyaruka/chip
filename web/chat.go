package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nyaruka/chip/v26/core/courier"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/vkutil"
)

const (
	chatIDLength = 24

	// how many chats a single IP can start on a channel per window - generous for a real visitor (who starts
	// one chat, ever) while capping how fast anyone can mint contacts
	startLimit       = 10
	startLimitWindow = time.Minute

	// how many messages a history request returns, and how many requests a single chat can make per window -
	// enough for a reconnecting client to catch up without letting anyone hammer the database
	historyPageSize    = 25
	historyLimit       = 10
	historyLimitWindow = time.Minute

	// how many status reports a single chat can make per window - a client acks each batch of messages it's
	// shown, so a handful covers reconnects and paging back through history
	statusLimit       = 20
	statusLimitWindow = time.Minute
)

var (
	startLimiter   = vkutil.NewRateLimiter("chat-starts", startLimit, startLimitWindow)
	historyLimiter = vkutil.NewRateLimiter("chat-history", historyLimit, historyLimitWindow)
	statusLimiter  = vkutil.NewRateLimiter("chat-statuses", statusLimit, statusLimitWindow)
)

var chatIDChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// channelHandler handles a request addressed to a webchat channel. An error it returns is one that isn't the
// caller's doing - anything that is, it answers itself.
type channelHandler func(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error

// channelRoute adapts a channel handler to an http.HandlerFunc: resolving the channel the request is addressed to,
// applying the channel's CORS policy, and answering with a 500 for an error the handler returns
func (s *Server) channelRoute(fn channelHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		uuid := chi.URLParam(r, "uuid")
		if !uuids.Is(uuid) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}

		ch, err := models.LoadChannel(ctx, s.rt.DB, models.ChannelUUID(uuid))
		if err != nil {
			logRequestError(r, "error loading channel", err)
			writeError(w, http.StatusInternalServerError, "error loading channel")
			return
		}
		if ch == nil {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}

		if !s.cors(ch, w, r) {
			return
		}

		if err := fn(ctx, ch, w, r); err != nil {
			logRequestError(r, "error handling request", err)
			writeError(w, http.StatusInternalServerError, "error handling request")
		}
	}
}

// cors enforces the channel's allowed domains and sets the CORS header that all responses on our public endpoints
// need - error responses included, or the widget's browser couldn't read them - because the widget calls from
// third-party origins. It returns false having answered the request if the origin isn't allowed.
//
// Allowed domains are an anti-embedding control for real browsers, which send a genuine Origin header - not an
// anti-abuse control, since scripted clients can spoof or omit Origin freely (the start rate limit and edge
// protection cover those) - so a request without an Origin header always passes.
func (s *Server) cors(ch *models.Channel, w http.ResponseWriter, r *http.Request) bool {
	domains := ch.AllowedDomains()
	origin := r.Header.Get("Origin")

	if len(domains) > 0 {
		// every branch below picks the allow-origin value from the request's origin so caches must vary on it
		w.Header().Add("Vary", "Origin")
	}

	if len(domains) > 0 && origin != "" {
		if !originAllowed(origin, domains) {
			// deliberately no allow-origin header on this response, so the embedding page's browser also blocks
			// it from reading the error
			writeError(w, http.StatusForbidden, "origin not allowed")
			return false
		}

		// reflect the specific origin instead of * so only pages on allowed domains get readable responses
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	return true
}

// originAllowed parses an Origin header value (scheme://host[:port]) and checks whether its host[:port] matches one
// of the configured domains, case-insensitively
func originAllowed(origin string, domains []string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return slices.ContainsFunc(domains, func(d string) bool { return strings.EqualFold(d, u.Host) })
}

// originHost returns the host[:port] of the request's Origin header, or an empty string if it has none or it's not
// one we can parse (e.g. "null")
func originHost(r *http.Request) string {
	u, err := url.Parse(r.Header.Get("Origin"))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// handlePreflight answers CORS preflight requests to the channel endpoints. The channel deliberately isn't loaded
// here so preflights can't check allowed domains and stay permissive - that's fine because the browser is gated by
// the allow-origin header on the actual response, which does check them.
func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	// allowing Content-Type covers any value it takes, so this accommodates both the JSON endpoints and the
	// upload endpoint's multipart posts
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

// allow checks a rate limit for the given id, failing open so that a valkey problem doesn't stop visitors using
// their chats
func (s *Server) allow(ctx context.Context, limiter *vkutil.RateLimiter, id string) bool {
	vc := s.rt.VK.Get()
	defer vc.Close()

	allowed, err := limiter.Allow(ctx, vc, id)
	if err != nil {
		slog.Error("error checking rate limit", "error", err, "id", id)
		return true
	}
	return allowed
}

// relayCourier answers a visitor's request with what courier said about it, when courier refused it
func relayCourier(w http.ResponseWriter, resp *courier.Response) {
	writeError(w, resp.Status, resp.Error())
}

type startResponse struct {
	ChatID string `json:"chat_id"`
}

// handleStart is our handler for a visitor opening a chat for the first time: it mints them a new chat ID and has
// courier create the contact behind it.
//
// It's necessarily unauthenticated - it's what hands a brand new visitor their credential, and the channel UUID
// it's addressed by is public in the widget's JS - so each request creating a contact is an abuse surface. The
// per-IP throttle below caps how fast a single caller can mint contacts - but only if the IP is trustworthy: the
// RealIP middleware takes it from forwarded headers without a trusted-proxy allowlist, so this relies on the edge
// overwriting client-supplied headers. Spoofed-header and distributed floods are left to edge-level protection like
// any other unauthenticated endpoint.
func (s *Server) handleStart(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error {
	if !s.allow(ctx, startLimiter, fmt.Sprintf("%s|%s", ch.UUID(), requestIP(r))) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return nil
	}

	// a chat ID is a bearer credential - possession is the only thing that identifies a webchat visitor - so it
	// comes from a CSPRNG (24 chars of [a-zA-Z0-9] is ~143 bits of entropy)
	chatID := random.SecureString(chatIDLength, chatIDChars)

	resp, err := s.courier.Start(ctx, ch, chatID, originHost(r))
	if err != nil {
		logRequestError(r, "error starting chat", err)
		writeError(w, http.StatusBadGateway, "error starting chat")
		return nil
	}
	if !resp.OK() {
		relayCourier(w, resp)
		return nil
	}

	writeJSON(w, http.StatusOK, &startResponse{ChatID: chatID})
	return nil
}

type receivePayload struct {
	ChatID string `json:"chat_id" validate:"required"`
	// max counts runes, and we reject rather than truncate because we control the widget - an over-limit
	// message is a client bug or abuse. Text is optional when the message carries attachments.
	Text string `json:"text" validate:"required_without=Attachments,max=1000"`
	// attachments the visitor previously uploaded, as the content-type:url values the upload endpoint returned
	Attachments []string `json:"attachments" validate:"max=10"`
	// the widget's own ID for the message, so that a send it retries isn't recorded twice
	ExternalID string `json:"external_id" validate:"omitempty,max=64"`
}

type receiveResponse struct {
	MsgUUID string `json:"msg_uuid"`
}

// handleReceive is our handler for a message from a visitor. Possession of the chat ID is what authenticates the
// caller.
func (s *Server) handleReceive(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error {
	payload := &receivePayload{}
	if err := readJSON(w, r, payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil
	}
	if _, err := models.ChatURN(payload.ChatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return nil
	}

	// attachments may only reference files uploaded to the channel workspace's own attachment storage - accepting
	// arbitrary URLs would let anyone use the platform as a fetch proxy, and attach content we never vetted - and
	// may only carry the media types the upload endpoint allows, since the type label here is client-supplied and
	// is what's stored on the message
	prefix := s.uploadURLPrefix(ch)
	for _, att := range payload.Attachments {
		if ct, u := splitAttachment(att); !uploadTypeAllowed(ct) || !strings.HasPrefix(u, prefix) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid attachment: %s", att))
			return nil
		}
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, payload.ChatID)
	if err != nil {
		return err
	}
	if visitor == nil {
		writeError(w, http.StatusBadRequest, "unknown chat id")
		return nil
	}

	msg := &courier.MsgIn{ChatID: payload.ChatID, ExternalID: payload.ExternalID, Text: payload.Text, Attachments: payload.Attachments}

	resp, err := s.courier.Receive(ctx, ch, msg)
	if err != nil {
		logRequestError(r, "error sending message", err)
		writeError(w, http.StatusBadGateway, "error sending message")
		return nil
	}
	if !resp.OK() {
		relayCourier(w, resp)
		return nil
	}

	writeJSON(w, http.StatusOK, &receiveResponse{MsgUUID: resp.MsgUUID()})
	return nil
}

// splitAttachment splits an attachment value into its content type and URL
func splitAttachment(att string) (string, string) {
	parts := strings.SplitN(att, ":", 2)
	if len(parts) < 2 {
		return "", att
	}
	return parts[0], parts[1]
}

type statusPayload struct {
	ChatID string `json:"chat_id" validate:"required"`
	// the statuses a chat client can report for the messages it's been sent - never failure, which is ours to decide
	Status string `json:"status"  validate:"required,oneof=delivered read"`
	// a client reports in batches - e.g. everything a history fetch returned - capped at a page of history
	MsgUUIDs []string `json:"msg_uuids" validate:"required,min=1,max=25,unique,dive,uuid"`
}

// handleStatus is our handler for a visitor's browser reporting that it received or displayed messages it was sent.
// Like receive, possession of the chat ID is what authenticates the caller.
func (s *Server) handleStatus(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error {
	payload := &statusPayload{}
	if err := readJSON(w, r, payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil
	}

	// validated before the throttle so malformed chat IDs can't mint valkey keys or share one empty-ID key
	if _, err := models.ChatURN(payload.ChatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return nil
	}

	// throttled per chat like history
	if !s.allow(ctx, statusLimiter, fmt.Sprintf("%s|%s", ch.UUID(), payload.ChatID)) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return nil
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, payload.ChatID)
	if err != nil {
		return err
	}
	if visitor == nil {
		writeError(w, http.StatusBadRequest, "unknown chat id")
		return nil
	}

	// UUIDs aren't checked against this chat's conversation: courier's status writes only touch outgoing messages
	// on the channel, and message UUIDs are unguessable, so like any status callback the exposure is a leaked UUID
	status := &courier.StatusUpdate{ChatID: payload.ChatID, Status: payload.Status, MsgUUIDs: payload.MsgUUIDs}

	resp, err := s.courier.Status(ctx, ch, status)
	if err != nil {
		logRequestError(r, "error reporting status", err)
		writeError(w, http.StatusBadGateway, "error reporting status")
		return nil
	}
	if !resp.OK() {
		relayCourier(w, resp)
		return nil
	}

	writeJSON(w, http.StatusOK, struct{}{})
	return nil
}

// historyResponse is the response to a history request: a page of message events, newest first, and - when the
// page was full - the cursor that fetches the next page back
type historyResponse struct {
	Events []*msgEvent `json:"events"`
	Next   string      `json:"next,omitempty"`
}

// handleHistory is our handler for a chat client fetching the recent messages in its conversation. Socket publishes
// are dropped when the visitor doesn't have the chat open, so a client that reconnects calls this to recover what
// it missed. Like receive, possession of the chat ID is what authenticates the caller.
func (s *Server) handleHistory(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error {
	chatID := r.URL.Query().Get("chat_id")

	// the paging cursor is a message UUID - v7, so UUID order is message order
	var before models.MsgUUID
	if v := r.URL.Query().Get("before"); v != "" {
		if !uuids.Is(v) {
			writeError(w, http.StatusBadRequest, "invalid before parameter")
			return nil
		}
		before = models.MsgUUID(v)
	}

	// validated before the throttle so malformed chat IDs can't mint valkey keys or share one empty-ID key
	if _, err := models.ChatURN(chatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return nil
	}

	// throttled per chat rather than per IP - a real client only needs a small burst at reconnect, and like
	// start, anything distributed is left to edge protection
	if !s.allow(ctx, historyLimiter, fmt.Sprintf("%s|%s", ch.UUID(), chatID)) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return nil
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, chatID)
	if err != nil {
		return err
	}
	if visitor == nil {
		writeError(w, http.StatusBadRequest, "unknown chat id")
		return nil
	}

	msgs, err := models.GetChatMsgs(ctx, s.rt.DB, ch, visitor, before, historyPageSize)
	if err != nil {
		return err
	}

	resp := &historyResponse{Events: make([]*msgEvent, len(msgs))}
	for i, m := range msgs {
		typ := eventTypeMsgOut
		if m.Direction == models.MsgIncoming {
			typ = eventTypeMsgIn
		}
		resp.Events[i] = &msgEvent{
			Type:         typ,
			CreatedOn:    m.CreatedOn.UTC(), // live events are stamped in UTC so history should look the same
			MsgUUID:      m.UUID,
			Text:         m.Text,
			Attachments:  m.Attachments,
			QuickReplies: models.TextQuickReplies(m.QuickReplies),
			User:         newMsgUser(s.rt, m.Sender()),
		}
	}

	// a full page may have older messages behind it, and its oldest item's UUID is the cursor to them
	if len(msgs) == historyPageSize {
		resp.Next = string(msgs[len(msgs)-1].UUID)
	}

	writeJSON(w, http.StatusOK, resp)
	return nil
}
