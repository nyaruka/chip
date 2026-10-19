// Package courier is chip's client for courier, which is where everything a visitor does ends up: courier creates
// the contact behind a new chat, writes the visitor's messages and their delivery and read reports, and hands them
// on to the rest of the platform. Chip talks to courier's webchat channel endpoints the way any messaging provider
// talks to its channel's endpoints, except that those endpoints only answer to chip.
package courier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/jsonx"
)

// the most of a courier response body we'll read - they're small JSON documents
const maxResponseBytes = 64 * 1024

// MsgIn is a message from a visitor, as posted to courier. ExternalID is the ID the widget gave the message, so
// that a send retried after a timeout is recognized by courier as the same message rather than written twice.
type MsgIn struct {
	ChatID      string   `json:"chat_id"`
	ExternalID  string   `json:"external_id,omitempty"`
	Text        string   `json:"text"`
	Attachments []string `json:"attachments,omitempty"`
}

// StatusUpdate is a visitor's report that messages reached their browser (delivered) or were shown to them (read)
type StatusUpdate struct {
	ChatID   string   `json:"chat_id"`
	Status   string   `json:"status"`
	MsgUUIDs []string `json:"msg_uuids"`
}

// Response is what courier answered: its status code and body, which the caller interprets - a 2xx is success and
// anything else is courier's reason for refusing, which a visitor-facing endpoint relays
type Response struct {
	Status int
	Body   []byte
}

// OK returns whether the response is a success
func (r *Response) OK() bool { return r.Status/100 == 2 }

// courier answers a received message or status with a list of what it did with each item in the request - which
// for chip is always a single item - and an error with a list of what went wrong
type dataResponse struct {
	Message string `json:"message"`
	Data    []struct {
		Type    string `json:"type"`
		MsgUUID string `json:"msg_uuid"`
		Error   string `json:"error"`
	} `json:"data"`
}

// MsgUUID returns the UUID courier gave a received message
func (r *Response) MsgUUID() string {
	d := &dataResponse{}
	if err := json.Unmarshal(r.Body, d); err == nil && len(d.Data) > 0 {
		return d.Data[0].MsgUUID
	}
	return ""
}

// Error returns courier's description of why it refused the request, or a generic one if the response isn't one
// we can read
func (r *Response) Error() string {
	d := &dataResponse{}
	if err := json.Unmarshal(r.Body, d); err == nil && len(d.Data) > 0 && d.Data[0].Error != "" {
		return d.Data[0].Error
	}
	return fmt.Sprintf("courier returned %d", r.Status)
}

// Client is the interface for talking to courier, real or mocked
type Client interface {
	// Start has courier create the contact behind a new chat. Origin is the host of the site the chat was started
	// from, so that a channel serving several sites can tell them apart.
	Start(ctx context.Context, ch *models.Channel, chatID, origin string) (*Response, error)

	// Receive hands courier a message from a visitor
	Receive(ctx context.Context, ch *models.Channel, msg *MsgIn) (*Response, error)

	// Status hands courier a visitor's delivery or read report
	Status(ctx context.Context, ch *models.Channel, status *StatusUpdate) (*Response, error)
}

type client struct {
	rt *runtime.Runtime
}

// NewClient creates a new client for the courier the runtime is configured with
func NewClient(rt *runtime.Runtime) Client {
	return &client{rt: rt}
}

type startPayload struct {
	ChatID string `json:"chat_id"`
	Origin string `json:"origin,omitempty"`
}

func (c *client) Start(ctx context.Context, ch *models.Channel, chatID, origin string) (*Response, error) {
	return c.post(ctx, ch, "start", &startPayload{ChatID: chatID, Origin: origin})
}

func (c *client) Receive(ctx context.Context, ch *models.Channel, msg *MsgIn) (*Response, error) {
	return c.post(ctx, ch, "receive", msg)
}

func (c *client) Status(ctx context.Context, ch *models.Channel, status *StatusUpdate) (*Response, error) {
	return c.post(ctx, ch, "status", status)
}

// post posts the given payload to one of the channel's endpoints on courier. An error is only a failure to get an
// answer - what courier answered is the response, whatever its status.
func (c *client) post(ctx context.Context, ch *models.Channel, action string, payload any) (*Response, error) {
	url := fmt.Sprintf("%s/c/wch/%s/%s", c.rt.Config.CourierURL, ch.UUID(), action)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonx.MustMarshal(payload)))
	if err != nil {
		return nil, fmt.Errorf("error creating courier request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := c.rt.Config.CourierAuthToken; token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}

	resp, err := c.rt.HTTP.Courier.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling courier: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("error reading courier response: %w", err)
	}

	return &Response{Status: resp.StatusCode, Body: body}, nil
}
