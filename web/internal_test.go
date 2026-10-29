package web

import (
	"errors"
	"testing"
	"time"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/vkutil/assertvk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	rt, s, _ := setup(t)

	dates.SetNowFunc(dates.NewFixedNow(time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC)))
	defer dates.SetNowFunc(time.Now)

	socket := models.ChatSocket(channelUUID, testChatID)
	send := func(body string) *response { return internal(s, "/send", "Token sesame", body) }

	msg := `"channel_uuid": "` + string(channelUUID) + `", "chat_id": "` + testChatID + `", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b"`

	// no subscriber on the socket yet, so the publish is dropped but the send still succeeds
	resp := send(`{` + msg + `, "text": "Hello there"}`)
	assert.Equal(t, 200, resp.Code, resp.Body)
	assert.JSONEq(t, `{}`, resp.Body)
	assert.Empty(t, testsuite.Publications(t, rt, socket))

	// mark the socket subscribed (as the subscribe proxy would)
	vc := rt.VK.Get()
	_, err := vc.Do("SET", centrifugo.SubscriptionKey(socket), "1")
	vc.Close()
	require.NoError(t, err)

	// a plain text message publishes just the envelope and text - no attachments or quick replies fields
	assert.Equal(t, 200, send(`{`+msg+`, "text": "Hello there"}`).Code)
	pubs := testsuite.Publications(t, rt, socket)
	require.Len(t, pubs, 1)
	assert.JSONEq(t, `{"type": "msg_out", "created_on": "2025-10-13T11:20:30Z", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hello there"}`, string(pubs[0]))

	// a message sent by a user carries who they are, as courier resolved them
	assert.Equal(t, 200, send(`{`+msg+`, "text": "Hello there", "user": {"uuid": "`+adminUUID+`", "name": "Andy Admin", "avatar": "http://s3:8333/test-default/avatars/andy.jpg"}}`).Code)
	pubs = testsuite.Publications(t, rt, socket)
	require.Len(t, pubs, 2)
	assert.JSONEq(t, `{
		"type": "msg_out", "created_on": "2025-10-13T11:20:30Z", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hello there",
		"user": {"uuid": "`+adminUUID+`", "name": "Andy Admin", "avatar": "http://s3:8333/test-default/avatars/andy.jpg"}
	}`, string(pubs[1]))

	// a message with attachments and quick replies includes them - except non-text quick replies, which the
	// widget doesn't render
	assert.Equal(t, 200, send(`{`+msg+`, "text": "Pick one", "attachments": ["image/jpeg:https://example.com/cat.jpg", "audio/mp3:https://example.com/hi.mp3"],
		"quick_replies": [{"type": "text", "text": "Yes"}, {"type": "url", "text": "More", "extra": "https://example.com"}]}`).Code)
	pubs = testsuite.Publications(t, rt, socket)
	require.Len(t, pubs, 3)
	assert.JSONEq(t, `{
		"type": "msg_out", "created_on": "2025-10-13T11:20:30Z", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Pick one",
		"attachments": ["image/jpeg:https://example.com/cat.jpg", "audio/mp3:https://example.com/hi.mp3"],
		"quick_replies": [{"type": "text", "text": "Yes"}]
	}`, string(pubs[2]))

	// the contact's email is accepted but never shown to the chat
	assert.Equal(t, 200, send(`{`+msg+`, "text": "Hello there", "email": "bob@example.com"}`).Code)
	pubs = testsuite.Publications(t, rt, socket)
	require.Len(t, pubs, 4)
	assert.JSONEq(t, `{"type": "msg_out", "created_on": "2025-10-13T11:20:30Z", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hello there"}`, string(pubs[3]))

	// bad requests: a missing or malformed channel, chat ID or message UUID
	for _, body := range []string{
		`{"chat_id": "` + testChatID + `", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hi"}`,
		`{"channel_uuid": "` + string(channelUUID) + `", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hi"}`,
		`{"channel_uuid": "` + string(channelUUID) + `", "chat_id": "nope", "msg_uuid": "0191e180-7d60-7000-aded-7d8b151cbd5b", "text": "Hi"}`,
		`{"channel_uuid": "` + string(channelUUID) + `", "chat_id": "` + testChatID + `", "msg_uuid": "0191e180", "text": "Hi"}`,
		`{"channel_uuid": "` + string(channelUUID) + `", "chat_id": "` + testChatID + `", "text": "Hi"}`,
		`not json`,
	} {
		assert.Equal(t, 400, send(body).Code, body)
	}
	assert.Len(t, testsuite.Publications(t, rt, socket), 4)

	// the channel doesn't send email, so nothing was held back for it
	vc = rt.VK.Get()
	assertvk.ZCard(t, vc, "chat-email-due", 0)
	vc.Close()

	// a publish failure is an error, for courier to retry
	rt.Centrifugo.Client.(*centrifugo.MockClient).SetError(errors.New("boom"))
	resp = send(`{` + msg + `, "text": "Hello there"}`)
	assert.Equal(t, 500, resp.Code)
	assert.JSONEq(t, `{"error": "error publishing event"}`, resp.Body)
}

func TestSendHoldsForEmail(t *testing.T) {
	rt, s, _ := setup(t)

	vc := rt.VK.Get()
	defer vc.Close()

	testdb.InsertChannel(t, rt, testdb.Org1, otherChannelUUID, map[string]any{
		"email_from": "help@example.com", "mailgun_domain": "mg.example.com", "mailgun_api_key": "key-1",
	})

	send := func(body string) *response { return internal(s, "/send", "Token sesame", body) }
	msg := func(uuid string) string {
		return `"channel_uuid": "` + string(otherChannelUUID) + `", "chat_id": "` + testChatID + `", "msg_uuid": "` + uuid + `", "text": "Hi"`
	}
	member := string(otherChannelUUID) + ":" + testChatID

	// a message to a contact with an email, on a channel that sends email, is held back once delivered
	assert.Equal(t, 200, send(`{`+msg("0191e180-7d60-7000-aded-7d8b151cbd5b")+`, "email": "bob@example.com"}`).Code)
	assert.Equal(t, 200, send(`{`+msg("0191e180-7d60-7000-aded-7d8b151cbd5c")+`, "email": "bob@example.com"}`).Code)
	assertvk.ZCard(t, vc, "chat-email-due", 1)
	assertvk.SMembers(t, vc, "chat-email-msgs:"+member, []string{"0191e180-7d60-7000-aded-7d8b151cbd5b", "0191e180-7d60-7000-aded-7d8b151cbd5c"})
	assertvk.HGet(t, vc, "chat-email:"+member, "email", "bob@example.com")

	// one without an email means the contact no longer has one, so nothing is held back for them
	assert.Equal(t, 200, send(`{`+msg("0191e180-7d60-7000-aded-7d8b151cbd5d")+`}`).Code)
	assertvk.ZCard(t, vc, "chat-email-due", 0)
	assertvk.NotExists(t, vc, "chat-email-msgs:"+member)

	// and one that isn't delivered isn't held back either
	_, err := vc.Do("SET", centrifugo.SubscriptionKey(models.ChatSocket(otherChannelUUID, testChatID)), "1")
	require.NoError(t, err)
	rt.Centrifugo.Client.(*centrifugo.MockClient).SetError(errors.New("boom"))
	assert.Equal(t, 500, send(`{`+msg("0191e180-7d60-7000-aded-7d8b151cbd5e")+`, "email": "bob@example.com"}`).Code)
	assertvk.ZCard(t, vc, "chat-email-due", 0)
}

func TestEvent(t *testing.T) {
	rt, s, _ := setup(t)

	dates.SetNowFunc(dates.NewFixedNow(time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC)))
	defer dates.SetNowFunc(time.Now)

	socket := models.ChatSocket(channelUUID, testChatID)
	vc := rt.VK.Get()
	_, err := vc.Do("SET", centrifugo.SubscriptionKey(socket), "1")
	vc.Close()
	require.NoError(t, err)

	send := func(body string) *response { return internal(s, "/event", "Token sesame", body) }

	chat := `"channel_uuid": "` + string(channelUUID) + `", "chat_id": "` + testChatID + `"`

	// typing by a user is attributed to them - by UUID and name only, like the platform's own events...
	resp := send(`{` + chat + `, "type": "typing_started", "user": {"uuid": "` + adminUUID + `", "name": "Andy Admin", "avatar": "http://example.com/andy.jpg"}}`)
	assert.Equal(t, 200, resp.Code, resp.Body)

	// ...and anything else isn't
	assert.Equal(t, 200, send(`{`+chat+`, "type": "typing_stopped"}`).Code)

	pubs := testsuite.Publications(t, rt, socket)
	require.Len(t, pubs, 2)
	assert.JSONEq(t, `{"type": "typing_started", "created_on": "2025-10-13T11:20:30Z", "user": {"uuid": "`+adminUUID+`", "name": "Andy Admin"}}`, string(pubs[0]))
	assert.JSONEq(t, `{"type": "typing_stopped", "created_on": "2025-10-13T11:20:30Z"}`, string(pubs[1]))

	// other event types aren't sendable
	resp = send(`{` + chat + `, "type": "contact_name_changed"}`)
	assert.Equal(t, 400, resp.Code)
	assert.Contains(t, resp.Body, "invalid request")

	// and a publish failure is an error
	rt.Centrifugo.Client.(*centrifugo.MockClient).SetError(errors.New("boom"))
	resp = send(`{` + chat + `, "type": "typing_started"}`)
	assert.Equal(t, 500, resp.Code)
	assert.JSONEq(t, `{"error": "error publishing event"}`, resp.Body)
}
