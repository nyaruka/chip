package email_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/nyaruka/chip/v26/core/email"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	tz, err := time.LoadLocation("Africa/Kigali")
	require.NoError(t, err)

	msgs := []*models.ChatMsg{
		{
			UUID:      "0191e180-7d60-7000-aded-7d8b151cbd5b",
			Direction: models.MsgOutgoing,
			Text:      "Hi there!\nHow can we help <you>?",
			CreatedOn: time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC),
		},
		{
			UUID:           "0191e180-7d60-7000-aded-7d8b151cbd5c",
			Direction:      models.MsgOutgoing,
			Text:           "Here's our menu",
			Attachments:    []string{"image/jpeg:https://example.com/menu.jpg", "application/pdf:https://example.com/prices.pdf"},
			QuickReplies:   models.QuickReplies{{Type: "text", Text: "Pizza"}, {Type: "text", Text: "Pasta"}, {Type: "location"}},
			CreatedOn:      time.Date(2025, 10, 13, 11, 21, 0, 0, time.UTC),
			UserID_:        sql.NullInt64{Int64: 1, Valid: true},
			UserFirstName_: sql.NullString{String: "Andy", Valid: true},
			UserLastName_:  sql.NullString{String: "Admin", Valid: true},
		},
	}

	e, err := email.Render("bob@example.com", "Nyaruka", tz, msgs)
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", e.To)
	assert.Equal(t, "New messages from Nyaruka", e.Subject)
	assert.Equal(t, `Nyaruka (Oct 13, 13:20):
Hi there!
How can we help <you>?

Andy Admin (Oct 13, 13:21):
Here's our menu
https://example.com/menu.jpg
https://example.com/prices.pdf
Options: Pizza · Pasta

--
You're receiving this because you chatted with Nyaruka and haven't seen these messages yet. To reply, continue the chat on our website.
`, e.Text)

	// text is escaped with its line breaks kept, images are shown inline and other attachments linked by name
	assert.Contains(t, e.HTML, `<strong style="color: #222;">Nyaruka</strong> &middot; Oct 13, 13:20`)
	assert.Contains(t, e.HTML, `Hi there!<br>How can we help &lt;you&gt;?`)
	assert.Contains(t, e.HTML, `<strong style="color: #222;">Andy Admin</strong> &middot; Oct 13, 13:21`)
	assert.Contains(t, e.HTML, `<a href="https://example.com/menu.jpg"><img src="https://example.com/menu.jpg" alt="menu.jpg"`)
	assert.Contains(t, e.HTML, `<a href="https://example.com/prices.pdf">prices.pdf</a>`)
	assert.Contains(t, e.HTML, `Options: Pizza · Pasta`)

	// a single message, with only attachments
	e, err = email.Render("bob@example.com", "Nyaruka", time.UTC, []*models.ChatMsg{
		{Direction: models.MsgOutgoing, Attachments: []string{"image/png:https://example.com/map.png"}, CreatedOn: time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC)},
	})
	require.NoError(t, err)
	assert.Equal(t, "New message from Nyaruka", e.Subject)
	assert.Equal(t, `Nyaruka (Oct 13, 11:20):
https://example.com/map.png

--
You're receiving this because you chatted with Nyaruka and haven't seen these messages yet. To reply, continue the chat on our website.
`, e.Text)
	assert.NotContains(t, e.HTML, `line-height`) // no empty text block

	// attachment names come from the decoded path, and the HTML escapes names and URLs - neutralizing any that
	// aren't safe to link
	e, err = email.Render("bob@example.com", "Nyaruka", time.UTC, []*models.ChatMsg{
		{
			Direction:      models.MsgOutgoing,
			Attachments:    []string{"application/pdf:https://example.com/files/price%20list.pdf?X-Amz-Signature=abc", "text/html:javascript:alert(1)"},
			QuickReplies:   models.QuickReplies{{Type: "text", Text: "Yes"}, {Type: "text", Text: "No"}},
			CreatedOn:      time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC),
			UserID_:        sql.NullInt64{Int64: 1, Valid: true},
			UserFirstName_: sql.NullString{String: "<b>Andy</b>", Valid: true},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, e.HTML, `>price list.pdf</a>`)
	assert.Contains(t, e.HTML, `<strong style="color: #222;">&lt;b&gt;Andy&lt;/b&gt;</strong>`)
	assert.Contains(t, e.HTML, `<a href="#ZgotmplZ">`)
	assert.NotContains(t, e.HTML, `="javascript:`)
	assert.Contains(t, e.HTML, `Options: Yes · No`)
	assert.Equal(t, `<b>Andy</b> (Oct 13, 11:20):
https://example.com/files/price%20list.pdf?X-Amz-Signature=abc
javascript:alert(1)
Options: Yes · No

--
You're receiving this because you chatted with Nyaruka and haven't seen these messages yet. To reply, continue the chat on our website.
`, e.Text)
}
