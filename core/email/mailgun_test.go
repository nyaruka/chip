package email_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/nyaruka/chip/v26/core/email"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)

	var reqPath, reqUser, reqPass string
	var reqForm url.Values
	status, body := 200, `{"id": "<123@mg.example.com>", "message": "Queued. Thank you."}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath = r.URL.Path
		reqUser, reqPass, _ = r.BasicAuth()
		b, _ := io.ReadAll(r.Body)
		reqForm, _ = url.ParseQuery(string(b))
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	defer server.Close()

	defer func(urls map[string]string) { email.MailgunURLs = urls }(email.MailgunURLs)
	email.MailgunURLs = map[string]string{"us": server.URL + "/us", "eu": server.URL + "/eu"}

	ch := &models.Channel{UUID_: "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71"}
	cfg := &models.EmailConfig{From: "help@example.com", Domain: "mg.example.com", Region: "eu", APIKey: "key-1"}
	e := &email.Email{To: "bob@example.com", Subject: "New message from Nyaruka", Text: "Hi", HTML: "<p>Hi</p>"}

	err := email.Send(ctx, rt, ch, cfg, "Nyaruka", e)
	require.NoError(t, err)
	assert.Equal(t, "/eu/v3/mg.example.com/messages", reqPath)
	assert.Equal(t, "api", reqUser)
	assert.Equal(t, "key-1", reqPass)
	assert.Equal(t, url.Values{
		"from":           {`"Nyaruka" <help@example.com>`},
		"to":             {"bob@example.com"},
		"subject":        {"New message from Nyaruka"},
		"text":           {"Hi"},
		"html":           {"<p>Hi</p>"},
		"v:channel_uuid": {"0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71"},
	}, reqForm)

	// an unknown region is the default one
	cfg.Region = ""
	require.NoError(t, email.Send(ctx, rt, ch, cfg, "Nyaruka", e))
	assert.Equal(t, "/us/v3/mg.example.com/messages", reqPath)

	// Mailgun refusing is an error that says whether trying again could help
	for _, tc := range []struct {
		status    int
		retryable bool
	}{
		{401, false},
		{400, false},
		{404, false},
		{429, true},
		{500, true},
		{503, true},
	} {
		status, body = tc.status, `{"message": "nope"}`
		err := email.Send(ctx, rt, ch, cfg, "Nyaruka", e)

		var sendErr *email.SendError
		require.True(t, errors.As(err, &sendErr), tc.status)
		assert.Equal(t, tc.status, sendErr.Status)
		assert.Equal(t, `{"message": "nope"}`, sendErr.Body)
		assert.Equal(t, tc.retryable, email.IsRetryable(err), tc.status)
	}

	// as is not being able to reach it
	server.Close()
	err = email.Send(ctx, rt, ch, cfg, "Nyaruka", e)
	assert.ErrorContains(t, err, "error calling mailgun")
	assert.True(t, email.IsRetryable(err))
	assert.False(t, email.IsRetryable(nil))
}
