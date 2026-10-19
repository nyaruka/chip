package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStart(t *testing.T) {
	rt, s, mock := setup(t)

	// a new visitor gets a chat ID, minted here and given to courier to create the contact behind - along with
	// the site they came from
	resp := public(s, http.MethodPost, channelUUID, "start", "", `{}`, "41.23.45.67:1234", "https://www.example.com")
	assert.Equal(t, 200, resp.Code)
	assert.JSONEq(t, `{"chat_id": "`+testChatID+`"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	reqs := mock.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "start", reqs[0].Action)
	assert.Equal(t, string(channelUUID), reqs[0].ChannelUUID)
	assert.Equal(t, "Token opensesame", reqs[0].Auth)
	assert.JSONEq(t, `{"chat_id": "`+testChatID+`", "origin": "www.example.com"}`, string(reqs[0].Body))

	// a request from outside a browser has no origin to pass on
	mock.Clear()
	assert.Equal(t, 200, post(s, channelUUID, "start", `{}`).Code)
	assert.NotContains(t, string(mock.Requests()[0].Body), "origin")

	// courier refusing to create the contact - e.g. the workspace has hit its contact limit - is relayed as is
	mock.Respond("start", 422, `{"message": "Error", "data": [{"type": "error", "error": "workspace has reached its limit of 1 contacts"}]}`)
	resp = post(s, channelUUID, "start", `{}`)
	assert.Equal(t, 422, resp.Code)
	assert.JSONEq(t, `{"error": "workspace has reached its limit of 1 contacts"}`, resp.Body)

	// as is a refusal we can't read
	mock.Respond("start", 500, `oops`)
	resp = post(s, channelUUID, "start", `{}`)
	assert.Equal(t, 500, resp.Code)
	assert.JSONEq(t, `{"error": "courier returned 500"}`, resp.Body)

	// and courier being unreachable is a bad gateway
	rt.Config.CourierURL = "http://127.0.0.1:1"
	resp = post(s, channelUUID, "start", `{}`)
	assert.Equal(t, 502, resp.Code)
	assert.JSONEq(t, `{"error": "error starting chat"}`, resp.Body)
}

func TestStartRateLimit(t *testing.T) {
	rt, s, mock := setup(t)
	testdb.InsertChannel(t, rt, testdb.Org1, otherChannelUUID, nil)

	startFrom := func(ch models.ChannelUUID, ip string) *response {
		return public(s, http.MethodPost, ch, "start", "", `{}`, ip, "")
	}

	// an IP can start up to the limit of chats within the window...
	for i := range startLimit {
		assert.Equal(t, 200, startFrom(channelUUID, "41.23.45.67:1234").Code, "start %d", i)
	}

	// ...then gets throttled, with the CORS header still on the error so the widget can read it
	resp := startFrom(channelUUID, "41.23.45.67:1234")
	assert.Equal(t, 429, resp.Code)
	assert.JSONEq(t, `{"error": "rate limit exceeded"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// without courier being asked for a contact
	assert.Len(t, mock.Requests(), startLimit)

	// but other IPs aren't affected
	assert.Equal(t, 200, startFrom(channelUUID, "41.23.45.68:1234").Code)

	// nor is the same IP on a different channel - the limit is scoped per channel
	assert.Equal(t, 200, startFrom(otherChannelUUID, "41.23.45.67:1234").Code)

	// and the count expires with the window
	assertTTL(t, rt, "chat-starts:"+string(channelUUID)+"|41.23.45.67", startLimitWindow)
}

// assertTTL checks that the given key exists and expires within the given window
func assertTTL(t *testing.T, rt *runtime.Runtime, key string, window time.Duration) {
	t.Helper()

	vc := rt.VK.Get()
	defer vc.Close()

	ttl, err := redis.Int(vc.Do("TTL", key))
	require.NoError(t, err)
	assert.Greater(t, ttl, 0, key)
	assert.LessOrEqual(t, ttl, int(window.Seconds()), key)
}

func TestCORS(t *testing.T) {
	_, s, _ := setup(t)

	// preflights on all the endpoints are answered without needing the channel
	for _, action := range []string{"start", "receive", "status", "history", "upload"} {
		resp := public(s, http.MethodOptions, "11111111-2222-3333-4444-555555555555", action, "", "", "", "https://evil.com")
		assert.Equal(t, 204, resp.Code, action)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"), action)
		assert.Equal(t, "GET, POST, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"), action)
		assert.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"), action)
	}

	// actual responses carry the allow-origin header too, including error responses
	resp := post(s, channelUUID, "start", `{}`)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	resp = post(s, channelUUID, "receive", `{}`)
	assert.Equal(t, 400, resp.Code)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestAllowedDomains(t *testing.T) {
	rt, s, mock := setup(t)

	const cfgChannelUUID models.ChannelUUID = "7d3fb8a2-5c1e-4b9f-a6d4-2e8c0f7b5a19"
	testdb.InsertChannel(t, rt, testdb.Org1, cfgChannelUUID, map[string]any{"allowed_domains": []string{"example.com", "localhost:3000"}})

	startFrom := func(ch models.ChannelUUID, origin string) *response {
		return public(s, http.MethodPost, ch, "start", "", `{}`, "", origin)
	}

	// a channel without allowed_domains ignores the origin and keeps the wildcard header
	resp := startFrom(channelUUID, "https://anywhere.com")
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// an allowed origin gets itself reflected back instead of the wildcard, with Vary: Origin
	resp = startFrom(cfgChannelUUID, "https://example.com")
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", resp.Header.Get("Vary"))

	// host matching is case-insensitive, and entries can carry a port
	resp = startFrom(cfgChannelUUID, "https://EXAMPLE.com")
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "https://EXAMPLE.com", resp.Header.Get("Access-Control-Allow-Origin"))
	resp = startFrom(cfgChannelUUID, "http://localhost:3000")
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "http://localhost:3000", resp.Header.Get("Access-Control-Allow-Origin"))

	mock.Clear()

	// a disallowed origin gets a 403 with no allow-origin header, so the embedding page can't read it either
	for _, origin := range []string{"https://evil.com", "https://sub.example.com", "https://example.com:8080", "null"} {
		resp = startFrom(cfgChannelUUID, origin)
		assert.Equal(t, 403, resp.Code, origin)
		assert.JSONEq(t, `{"error": "origin not allowed"}`, resp.Body, origin)
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"), origin)
	}

	// on the other endpoints too
	for _, action := range []string{"receive", "status", "upload"} {
		resp = public(s, http.MethodPost, cfgChannelUUID, action, "", `{}`, "", "https://evil.com")
		assert.Equal(t, 403, resp.Code, action)
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"), action)
	}
	resp = public(s, http.MethodGet, cfgChannelUUID, "history", "?chat_id=abcdefabcdefabcdefabcdef", "", "", "https://evil.com")
	assert.Equal(t, 403, resp.Code)
	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))

	// with an allowed origin reflected even on an error response, so the widget can read it
	resp = public(s, http.MethodGet, cfgChannelUUID, "history", "?chat_id=abcdefabcdefabcdefabcdef", "", "", "https://example.com")
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unknown chat id"}`, resp.Body)
	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))

	// without courier having been asked for any contacts by the blocked starts
	assert.Empty(t, mock.Requests())

	// but a request with no Origin header (a non-browser client) passes, still marked as varying on origin
	resp = startFrom(cfgChannelUUID, "")
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", resp.Header.Get("Vary"))
}

func TestReceive(t *testing.T) {
	rt, s, mock := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	// a message is handed to courier with the visitor's chat ID and the widget's own ID for it, and the visitor
	// gets back the UUID courier gave it
	resp := post(s, channelUUID, "receive", `{"chat_id": "`+testChatID+`", "text": "Hello", "external_id": "w1"}`)
	assert.Equal(t, 200, resp.Code, resp.Body)

	reqs := mock.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "receive", reqs[0].Action)
	assert.Equal(t, "Token opensesame", reqs[0].Auth)
	assert.JSONEq(t, `{"chat_id": "`+testChatID+`", "external_id": "w1", "text": "Hello"}`, string(reqs[0].Body))

	r := &receiveResponse{}
	resp.JSON(t, r)
	assert.Len(t, r.MsgUUID, 36)

	// attachments are passed on if they're uploads to this workspace's storage, of a type visitors may upload
	att := "image/jpeg:http://s3:8333/test-attachments/attachments/1/0e5d/9a6b/0e5d9a6b-1111-4222-8333-444455556666.jpg"
	mock.Clear()
	resp = post(s, channelUUID, "receive", `{"chat_id": "`+testChatID+`", "attachments": ["`+att+`"]}`)
	assert.Equal(t, 200, resp.Code, resp.Body)
	assert.JSONEq(t, `{"chat_id": "`+testChatID+`", "text": "", "attachments": ["`+att+`"]}`, string(mock.Requests()[0].Body))

	// bad requests: nothing to send, text too long, too many attachments, attachments from elsewhere or of other
	// types, a chat ID that's malformed or that we never minted
	mock.Clear()
	for body, err := range map[string]string{
		`{"chat_id": "` + testChatID + `"}`:                                                                                         "invalid request",
		`{"chat_id": "` + testChatID + `", "text": "` + string(make([]byte, 1001)) + `"}`:                                           "invalid JSON",
		`{"chat_id": "` + testChatID + `", "text": "` + fmt.Sprintf("%01001d", 0) + `"}`:                                            "invalid request",
		`{"chat_id": "` + testChatID + `", "attachments": [` + repeat(`"`+att+`"`, 11) + `]}`:                                       "invalid request",
		`{"chat_id": "` + testChatID + `", "attachments": ["image/jpeg:https://evil.com/cat.jpg"]}`:                                 "invalid attachment",
		`{"chat_id": "` + testChatID + `", "attachments": ["image/jpeg:http://s3:8333/test-attachments/attachments/2/cat.jpg"]}`:    "invalid attachment",
		`{"chat_id": "` + testChatID + `", "attachments": ["application/zip:http://s3:8333/test-attachments/attachments/1/x.zip"]}`: "invalid attachment",
		`{"chat_id": "` + testChatID + `", "attachments": ["` + att + `", "http://s3:8333/test-attachments/attachments/1/x.jpg"]}`:  "invalid attachment",
		`{"chat_id": "not-a-chat-id!", "text": "Hello"}`:                                                                            "invalid chat id",
		`{"chat_id": "xxxxxhDrqpTQefIEinK0up3C", "text": "Hello"}`:                                                                  "unknown chat id",
		`not json`: "invalid JSON",
	} {
		resp = post(s, channelUUID, "receive", body)
		assert.Equal(t, 400, resp.Code, body)
		assert.Contains(t, resp.Body, err, body)
	}
	assert.Empty(t, mock.Requests())

	// courier refusing a message is relayed
	mock.Respond("receive", 400, `{"message": "Error", "data": [{"type": "error", "error": "unknown chat id: `+testChatID+`"}]}`)
	resp = post(s, channelUUID, "receive", `{"chat_id": "`+testChatID+`", "text": "Hello"}`)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unknown chat id: `+testChatID+`"}`, resp.Body)

	// and courier being unreachable is a bad gateway
	rt.Config.CourierURL = "http://127.0.0.1:1"
	resp = post(s, channelUUID, "receive", `{"chat_id": "`+testChatID+`", "text": "Hello"}`)
	assert.Equal(t, 502, resp.Code)
	assert.JSONEq(t, `{"error": "error sending message"}`, resp.Body)
}

func repeat(s string, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = s
	}
	return join(parts, ",")
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func TestStatus(t *testing.T) {
	rt, s, mock := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	body := `{"chat_id": "` + testChatID + `", "status": "delivered", "msg_uuids": ["11f0a1d2-0000-7000-8000-000000000001", "11f0a1d2-0000-7000-8000-000000000002"]}`

	// a report is handed to courier as is
	resp := post(s, channelUUID, "status", body)
	assert.Equal(t, 200, resp.Code, resp.Body)
	assert.JSONEq(t, `{}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	reqs := mock.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "status", reqs[0].Action)
	assert.JSONEq(t, body, string(reqs[0].Body))

	// a client can report up to a page of history at once
	pageOfUUIDs := func(n int) string {
		uuids := make([]string, n)
		for i := range uuids {
			uuids[i] = fmt.Sprintf(`"11f0a1d2-0000-7000-8000-0000000001%02d"`, i)
		}
		return `{"chat_id": "` + testChatID + `", "status": "read", "msg_uuids": [` + join(uuids, ",") + `]}`
	}
	assert.Equal(t, 200, post(s, channelUUID, "status", pageOfUUIDs(historyPageSize)).Code)

	// bad requests: an unknown chat, a status a client can't report, and no, too many, malformed or duplicate
	// message UUIDs
	mock.Clear()
	for _, body := range []string{
		pageOfUUIDs(historyPageSize + 1),
		`{"chat_id": "xxxxxhDrqpTQefIEinK0up3C", "status": "read", "msg_uuids": ["11f0a1d2-0000-7000-8000-000000000002"]}`,
		`{"chat_id": "` + testChatID + `", "status": "failed", "msg_uuids": ["11f0a1d2-0000-7000-8000-000000000002"]}`,
		`{"chat_id": "` + testChatID + `", "status": "read", "msg_uuids": []}`,
		`{"chat_id": "` + testChatID + `", "status": "read", "msg_uuids": ["11f0a1d2"]}`,
		`{"chat_id": "` + testChatID + `", "status": "read", "msg_uuids": ["11f0a1d2-0000-7000-8000-000000000002", "11f0a1d2-0000-7000-8000-000000000002"]}`,
	} {
		assert.Equal(t, 400, post(s, channelUUID, "status", body).Code, body)
	}
	assert.Empty(t, mock.Requests())
}

func TestStatusRateLimit(t *testing.T) {
	rt, s, _ := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	report := func(chatID string) *response {
		return post(s, channelUUID, "status", `{"chat_id": "`+chatID+`", "status": "read", "msg_uuids": ["11f0a1d2-0000-7000-8000-000000000001"]}`)
	}

	// a chat can report statuses up to the limit of requests within the window...
	for i := range statusLimit {
		assert.Equal(t, 200, report(testChatID).Code, "request %d", i)
	}

	// ...then gets throttled, with the CORS header still on the error so the widget can read it
	resp := report(testChatID)
	assert.Equal(t, 429, resp.Code)
	assert.JSONEq(t, `{"error": "rate limit exceeded"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// but other chats aren't affected - the limit is per chat (an unknown one just fails its lookup)
	assert.Equal(t, 400, report("xxxxxhDrqpTQefIEinK0up3C").Code)

	// and the count expires with the window
	assertTTL(t, rt, "chat-statuses:"+string(channelUUID)+"|"+testChatID, statusLimitWindow)
}

func TestHistory(t *testing.T) {
	rt, s, _ := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	visitor := testdb.InsertVisitor(t, rt, ch, testChatID)
	otherChannel := testdb.InsertChannel(t, rt, testdb.Org1, otherChannelUUID, nil)

	admin, agent := testdb.UserID(t, rt, adminEmail), testdb.UserID(t, rt, agentEmail)
	testdb.SetAvatar(t, rt, admin, "avatars/andy.jpg")

	get := func(query string) *response {
		return public(s, http.MethodGet, channelUUID, "history", query, "", "", "")
	}

	day := time.Date(2025, 10, 13, 0, 0, 0, 0, time.UTC)
	// includes a non-text quick reply which should be filtered out of the history event
	quickReplies := `[{"type": "text", "text": "Yes"}, {"type": "text", "text": "No"}, {"type": "url", "text": "More", "extra": "https://example.com"}]`

	// the conversation: a plain outgoing message from a user with an avatar, an outgoing one with attachments and
	// quick replies from one without, and an incoming reply
	testdb.InsertMsg(t, rt, ch, visitor, "11f0a1d2-0000-7000-8000-200000000001", models.MsgOutgoing, "Hello", day.Add(11*time.Hour+1*time.Minute), testdb.MsgOptions{SentBy: admin})
	testdb.InsertMsg(t, rt, ch, visitor, "11f0a1d2-0000-7000-8000-200000000002", models.MsgOutgoing, "Pick one", day.Add(11*time.Hour+2*time.Minute), testdb.MsgOptions{
		Attachments: []string{"image/jpeg:https://example.com/cat.jpg"}, QuickReplies: quickReplies, SentBy: agent,
	})
	testdb.InsertMsg(t, rt, ch, visitor, "11f0a1d2-0000-7000-8000-200000000003", models.MsgIncoming, "Hi there", day.Add(11*time.Hour+3*time.Minute), testdb.MsgOptions{})

	// messages that shouldn't appear: deleted, archived, this chat on a different channel, and another chat
	otherVisitor := testdb.InsertVisitor(t, rt, ch, "aaaabbbbccccddddeeeeffff")
	testdb.InsertMsg(t, rt, ch, visitor, "11f0a1d2-0000-7000-8000-000000000004", models.MsgIncoming, "Deleted", day.Add(11*time.Hour+4*time.Minute), testdb.MsgOptions{Visibility: "D"})
	testdb.InsertMsg(t, rt, ch, visitor, "11f0a1d2-0000-7000-8000-000000000005", models.MsgOutgoing, "Archived", day.Add(11*time.Hour+5*time.Minute), testdb.MsgOptions{Visibility: "A"})
	testdb.InsertMsg(t, rt, otherChannel, visitor, "11f0a1d2-0000-7000-8000-000000000006", models.MsgOutgoing, "Other channel", day.Add(11*time.Hour+6*time.Minute), testdb.MsgOptions{})
	testdb.InsertMsg(t, rt, ch, otherVisitor, "11f0a1d2-0000-7000-8000-000000000007", models.MsgOutgoing, "Other chat", day.Add(11*time.Hour+7*time.Minute), testdb.MsgOptions{})

	// the whole conversation fits in one page, newest first, with outgoing messages shaped exactly like the
	// msg_out socket events and incoming ones as msg_in
	resp := get("?chat_id=" + testChatID)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.JSONEq(t, `{
		"events": [
			{"type": "msg_in", "created_on": "2025-10-13T11:03:00Z", "msg_uuid": "11f0a1d2-0000-7000-8000-200000000003", "text": "Hi there"},
			{
				"type": "msg_out",
				"created_on": "2025-10-13T11:02:00Z",
				"msg_uuid": "11f0a1d2-0000-7000-8000-200000000002",
				"text": "Pick one",
				"attachments": ["image/jpeg:https://example.com/cat.jpg"],
				"quick_replies": [{"type": "text", "text": "Yes"}, {"type": "text", "text": "No"}],
				"user": {"uuid": "`+agentUUID+`", "name": "Ann D'Agent"}
			},
			{
				"type": "msg_out",
				"created_on": "2025-10-13T11:01:00Z",
				"msg_uuid": "11f0a1d2-0000-7000-8000-200000000001",
				"text": "Hello",
				"user": {"uuid": "`+adminUUID+`", "name": "Andy Admin", "avatar": "http://s3:8333/test-default/avatars/andy.jpg"}
			}
		]
	}`, resp.Body)

	// add enough older messages - older UUIDs, since v7 UUID order is message order - that the conversation no
	// longer fits in one page
	oldUUIDs := make([]models.MsgUUID, 25)
	for i := range 25 {
		oldUUIDs[i] = models.MsgUUID(fmt.Sprintf("11f0a1d2-0000-7000-8000-1000000000%02d", i))
		testdb.InsertMsg(t, rt, ch, visitor, oldUUIDs[i], models.MsgIncoming, fmt.Sprintf("Old %d", i), day.Add(10*time.Hour+time.Duration(i)*time.Minute), testdb.MsgOptions{})
	}

	type page struct {
		Events []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"events"`
		Next string `json:"next"`
	}

	// the first page is the newest 25 messages, with the oldest one's UUID as the cursor to the rest
	resp = get("?chat_id=" + testChatID)
	assert.Equal(t, 200, resp.Code)
	p1 := &page{}
	resp.JSON(t, p1)
	require.Len(t, p1.Events, 25)
	assert.Equal(t, "Hi there", p1.Events[0].Text)
	assert.Equal(t, "Old 3", p1.Events[24].Text)
	assert.Equal(t, string(oldUUIDs[3]), p1.Next)

	// which fetches the remaining messages, which don't fill a page so there's no further cursor
	resp = get("?chat_id=" + testChatID + "&before=" + p1.Next)
	assert.Equal(t, 200, resp.Code)
	p2 := &page{}
	resp.JSON(t, p2)
	require.Len(t, p2.Events, 3)
	assert.Equal(t, "Old 2", p2.Events[0].Text)
	assert.Equal(t, "Old 1", p2.Events[1].Text)
	assert.Equal(t, "Old 0", p2.Events[2].Text)
	assert.Empty(t, p2.Next)

	// a chat ID we never minted is a bad request, as is a missing or malformed one, or a malformed cursor
	resp = get("?chat_id=xxxxxhDrqpTQefIEinK0up3C")
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unknown chat id"}`, resp.Body)
	resp = get("")
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "invalid chat id"}`, resp.Body)
	for _, before := range []string{"yesterday", "11f0a1d2", "11f0a1d2-0000-7000-8000-10000000000x"} {
		resp = get("?chat_id=" + testChatID + "&before=" + before)
		assert.Equal(t, 400, resp.Code, before)
		assert.JSONEq(t, `{"error": "invalid before parameter"}`, resp.Body, before)
	}

	// a released contact's chat is gone
	testdb.ReleaseContact(t, rt, visitor)
	resp = get("?chat_id=" + testChatID)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unknown chat id"}`, resp.Body)
}

func TestHistoryRateLimit(t *testing.T) {
	rt, s, _ := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	get := func(chatID string) *response {
		return public(s, http.MethodGet, channelUUID, "history", "?chat_id="+chatID, "", "", "")
	}

	// a chat can fetch history up to the limit of requests within the window...
	for i := range historyLimit {
		assert.Equal(t, 200, get(testChatID).Code, "request %d", i)
	}

	// ...then gets throttled, with the CORS header still on the error so the widget can read it
	resp := get(testChatID)
	assert.Equal(t, 429, resp.Code)
	assert.JSONEq(t, `{"error": "rate limit exceeded"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// but other chats aren't affected - the limit is per chat (an unknown one just fails its lookup)
	assert.Equal(t, 400, get("xxxxxhDrqpTQefIEinK0up3C").Code)

	// and the count expires with the window
	assertTTL(t, rt, "chat-history:"+string(channelUUID)+"|"+testChatID, historyLimitWindow)
}

// a JSON-decoding sanity check of the shapes the widget sees, so that a change to them is a deliberate one
func TestEventShapes(t *testing.T) {
	e := &msgEvent{Type: eventTypeMsgOut, MsgUUID: "11f0a1d2-0000-7000-8000-200000000001", Text: "Hi"}
	b, _ := json.Marshal(e)
	assert.JSONEq(t, `{"type": "msg_out", "created_on": "0001-01-01T00:00:00Z", "msg_uuid": "11f0a1d2-0000-7000-8000-200000000001", "text": "Hi"}`, string(b))
}
