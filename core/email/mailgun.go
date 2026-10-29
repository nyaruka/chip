// Package email is how chip emails visitors the messages they didn't see in the chat: rendering them, and sending
// through the Mailgun domain their channel is configured with.
package email

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
)

// the most of a Mailgun response body we'll read - just enough to log why it refused a message
const maxResponseBytes = 4 * 1024

// MailgunURLs are the base URLs of Mailgun's API by the region a domain belongs to, a var so tests can point it
// elsewhere
var MailgunURLs = map[string]string{
	"us": "https://api.mailgun.net",
	"eu": "https://api.eu.mailgun.net",
}

// Email is an email to a visitor, as rendered
type Email struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// ErrUnsendable is an email that can't be sent however often it's tried, without even asking Mailgun
var ErrUnsendable = errors.New("unsendable email")

// SendError is Mailgun refusing to send an email
type SendError struct {
	Status int
	Body   string
}

func (e *SendError) Error() string {
	return fmt.Sprintf("mailgun returned %d: %s", e.Status, e.Body)
}

// IsRetryable returns whether a send that failed with the given error may succeed if tried again: Mailgun being
// unreachable, rate limiting us or failing itself. Anything else it refuses - a bad key, domain or address - will be
// refused again.
func IsRetryable(err error) bool {
	if errors.Is(err, ErrUnsendable) {
		return false
	}
	var sendErr *SendError
	if errors.As(err, &sendErr) {
		return sendErr.Status == http.StatusTooManyRequests || sendErr.Status/100 == 5
	}
	return err != nil
}

// Send sends the given email from the given channel. The channel's key is a domain sending key, which only grants
// this one call.
func Send(ctx context.Context, rt *runtime.Runtime, ch *models.Channel, cfg *models.EmailConfig, fromName string, e *Email) error {
	region := strings.ToLower(cfg.Region)
	if region == "" {
		region = "us" // the region the platform defaults to
	}
	baseURL, ok := MailgunURLs[region]
	if !ok {
		return fmt.Errorf("%w: unknown mailgun region %q", ErrUnsendable, cfg.Region)
	}

	// Mailgun takes a list of recipients, but a visitor's email must only ever reach them
	to, err := mail.ParseAddress(e.To)
	if err != nil {
		return fmt.Errorf("%w: invalid recipient address", ErrUnsendable)
	}

	from := (&mail.Address{Name: fromName, Address: cfg.From}).String()

	form := url.Values{
		"from":    {from},
		"to":      {to.Address},
		"subject": {e.Subject},
		"text":    {e.Text},
		"html":    {e.HTML},

		// carried on Mailgun's events and webhooks for this message, so they can be matched to the channel
		"v:channel_uuid": {string(ch.UUID())},
	}

	endpoint := fmt.Sprintf("%s/v3/%s/messages", baseURL, url.PathEscape(cfg.Domain))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("error creating mailgun request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("api", cfg.APIKey)

	resp, err := rt.HTTP.Mailgun.Do(req)
	if err != nil {
		return fmt.Errorf("error calling mailgun: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return &SendError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}
