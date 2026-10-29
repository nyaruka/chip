package email_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/nyaruka/chip/v26/core/email"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/vkutil/assertvk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	chatID = "vM0GGhDrqpTQefIEinK0up3C"
	member = "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71:" + chatID

	msg1 models.MsgUUID = "0191e180-7d60-7000-aded-7d8b151cbd51"
	msg2 models.MsgUUID = "0191e180-7d60-7000-aded-7d8b151cbd52"
	msg3 models.MsgUUID = "0191e180-7d60-7000-aded-7d8b151cbd53"
)

// mockMailgun records the emails sent through it, answering with the given status
type mockMailgun struct {
	mu     sync.Mutex
	status int
	sent   []url.Values
}

func newMockMailgun(t *testing.T) *mockMailgun {
	m := &mockMailgun{status: 200}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(b))

		m.mu.Lock()
		defer m.mu.Unlock()
		m.sent = append(m.sent, form)
		w.WriteHeader(m.status)
	}))
	t.Cleanup(server.Close)

	urls := email.MailgunURLs
	email.MailgunURLs = map[string]string{"us": server.URL}
	t.Cleanup(func() { email.MailgunURLs = urls })
	return m
}

func (m *mockMailgun) respond(status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
}

// popSent returns the emails sent since it was last called
func (m *mockMailgun) popSent() []url.Values {
	m.mu.Lock()
	defer m.mu.Unlock()
	sent := m.sent
	m.sent = nil
	return sent
}

func TestSweeper(t *testing.T) {
	ctx, rt := testsuite.Runtime(t)
	mailgun := newMockMailgun(t)

	vc := rt.VK.Get()
	defer vc.Close()

	t0 := time.Date(2025, 10, 13, 11, 20, 0, 0, time.UTC)
	setNow := func(d time.Duration) { dates.SetNowFunc(dates.NewFixedNow(t0.Add(d))) }
	defer dates.SetNowFunc(time.Now)

	ch := testdb.InsertChannel(t, rt, testdb.Org1, "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71", map[string]any{
		"email_from": "help@example.com", "mailgun_domain": "mg.example.com", "mailgun_api_key": "key-1",
	})
	visitor := testdb.InsertVisitor(t, rt, ch, chatID)
	testdb.InsertMsg(t, rt, ch, visitor, msg1, models.MsgOutgoing, "Hi there", t0, testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, visitor, msg2, models.MsgOutgoing, "Are you there?", t0, testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, visitor, msg3, models.MsgOutgoing, "Bye", t0, testdb.MsgOptions{})

	sweeper := email.NewSweeper(rt)
	hold := func(d time.Duration, msg models.MsgUUID, address string) {
		setNow(d)
		require.NoError(t, email.Hold(ctx, rt, ch.UUID(), chatID, msg, address))
	}
	tick := func(d time.Duration) {
		setNow(d)
		require.NoError(t, sweeper.Tick(ctx))
	}
	assertNothingPending := func() {
		assertvk.ZCard(t, vc, "chat-email-due", 0)
		assertvk.NotExists(t, vc, "chat-email:"+member)
		assertvk.NotExists(t, vc, "chat-email-msgs:"+member)
	}

	// a held back message is due once the conversation has been quiet for a while, and each message held back
	// puts that off - the latest address winning
	hold(0, msg1, "old@example.com")
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(3*time.Minute).Unix()))
	hold(time.Minute, msg2, "bob@example.com")
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(4*time.Minute).Unix()))
	assertvk.SMembers(t, vc, "chat-email-msgs:"+member, []string{string(msg1), string(msg2)})
	assertvk.HGetAll(t, vc, "chat-email:"+member, map[string]string{"email": "bob@example.com", "first": "1760354400"})

	tick(3 * time.Minute)
	assert.Empty(t, mailgun.popSent())

	// once due, only what the visitor still hasn't read is emailed
	rt.DB.MustExec(`UPDATE msgs_msg SET status = 'R' WHERE uuid = $1`, msg1)
	tick(4 * time.Minute)
	sent := mailgun.popSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "bob@example.com", sent[0].Get("to"))
	assert.Equal(t, "New message from TextIt", sent[0].Get("subject"))
	assert.Contains(t, sent[0].Get("text"), "Are you there?")
	assert.NotContains(t, sent[0].Get("text"), "Hi there")
	assertNothingPending()

	// and a sweep that finds nothing due sends nothing
	tick(time.Hour)
	assert.Empty(t, mailgun.popSent())

	// a chatty conversation is only held back so long
	hold(time.Hour, msg3, "bob@example.com")
	hold(time.Hour+29*time.Minute, msg3, "bob@example.com")
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(time.Hour+30*time.Minute).Unix()))

	// if the visitor read everything held back, there's nothing to email
	rt.DB.MustExec(`UPDATE msgs_msg SET status = 'R' WHERE uuid = $1`, msg3)
	tick(2 * time.Hour)
	assert.Empty(t, mailgun.popSent())
	assertNothingPending()
	rt.DB.MustExec(`UPDATE msgs_msg SET status = 'W' WHERE uuid = $1`, msg3)

	// dropping a conversation's held back messages means they're never emailed
	hold(3*time.Hour, msg3, "bob@example.com")
	require.NoError(t, email.Drop(ctx, rt, ch.UUID(), chatID))
	assertNothingPending()

	// concurrent sweeps email a conversation once
	hold(4*time.Hour, msg3, "bob@example.com")
	setNow(5 * time.Hour)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { assert.NoError(t, email.NewSweeper(rt).Tick(ctx)) })
	}
	wg.Wait()
	assert.Len(t, mailgun.popSent(), 1)
	assertNothingPending()

	// a send that may succeed if tried again is, after a growing delay, until it's been retried enough times
	mailgun.respond(503)
	hold(6*time.Hour, msg3, "bob@example.com")
	tick(7 * time.Hour)
	assert.Len(t, mailgun.popSent(), 1)
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(7*time.Hour+time.Minute).Unix()))
	assertvk.SMembers(t, vc, "chat-email-msgs:"+member, []string{string(msg3)})
	assertvk.HGet(t, vc, "chat-email:"+member, "attempts", "1")

	tick(7*time.Hour + time.Minute)
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(7*time.Hour+6*time.Minute).Unix()))

	// holding back another message doesn't cut that wait short, and joins the batch being retried
	hold(7*time.Hour+2*time.Minute, msg2, "bob@example.com")
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(7*time.Hour+6*time.Minute).Unix()))
	assertvk.SMembers(t, vc, "chat-email-msgs:"+member, []string{string(msg2), string(msg3)})
	tick(7*time.Hour + 6*time.Minute)
	assertvk.ZScore(t, vc, "chat-email-due", member, float64(t0.Add(7*time.Hour+21*time.Minute).Unix()))
	assertvk.HGet(t, vc, "chat-email:"+member, "attempts", "3")
	tick(7*time.Hour + 21*time.Minute)
	assert.Len(t, mailgun.popSent(), 3)
	assertNothingPending()

	// a send that will fail again isn't retried
	mailgun.respond(401)
	hold(8*time.Hour, msg3, "bob@example.com")
	tick(9 * time.Hour)
	assert.Len(t, mailgun.popSent(), 1)
	assertNothingPending()

	// nor is one to an address that can't be emailed, which isn't even tried
	mailgun.respond(200)
	hold(10*time.Hour, msg3, "bob")
	tick(11 * time.Hour)
	assert.Empty(t, mailgun.popSent())
	assertNothingPending()

	// a conversation whose channel no longer sends email, or whose contact is gone, is skipped
	hold(12*time.Hour, msg3, "bob@example.com")
	rt.DB.MustExec(`UPDATE channels_channel SET config = '{}' WHERE id = $1`, ch.ID())
	tick(13 * time.Hour)
	assert.Empty(t, mailgun.popSent())
	assertNothingPending()

	rt.DB.MustExec(`UPDATE channels_channel SET config = '{"email_from": "help@example.com", "mailgun_domain": "mg.example.com", "mailgun_api_key": "key-1"}' WHERE id = $1`, ch.ID())
	hold(14*time.Hour, msg3, "bob@example.com")
	testdb.ReleaseContact(t, rt, visitor)
	tick(15 * time.Hour)
	assert.Empty(t, mailgun.popSent())
	assertNothingPending()
}

func TestSweeperStartStop(t *testing.T) {
	_, rt := testsuite.Runtime(t)

	sweeper := email.NewSweeper(rt)
	sweeper.Start()
	sweeper.Stop()
}
