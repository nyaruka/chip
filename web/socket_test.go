package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/gocommon/dates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proxy makes a request of one of the subscription proxies, as the realtime server would, for the given socket
// from the given origin
func proxy(s *Server, which, socket, origin string) *response {
	body := `{"client": "9336a229-2400-4382-8d27-9ec18b28219c", "user": "", "channel": "` + socket + `"}`
	if socket == "<missing>" {
		body = `{"client": "9336a229-2400-4382-8d27-9ec18b28219c", "user": ""}`
	} else if socket == "<number>" {
		body = `{"client": "9336a229-2400-4382-8d27-9ec18b28219c", "user": "", "channel": 123}`
	}
	req := httptest.NewRequest(http.MethodPost, "https://localhost"+internalPrefix+"/socket/"+which, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token sesame")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return request(s.InternalHandler(), req)
}

func TestSubscriptionProxies(t *testing.T) {
	rt, s, _ := setup(t)

	dates.SetNowFunc(dates.NewFixedNow(time.Date(2025, 10, 13, 11, 20, 30, 0, time.UTC)))
	defer dates.SetNowFunc(time.Now)

	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	visitor := testdb.InsertVisitor(t, rt, ch, testChatID)

	// a webchat channel in another workspace with its own visitor
	const otherChatID = "aB3dEf6hIj9kLm2nOp5qRs8t"
	otherChannel := testdb.InsertChannel(t, rt, testdb.Org2, otherChannelUUID, nil)
	testdb.InsertVisitor(t, rt, otherChannel, otherChatID)

	// a channel pinned to its operator's own sites
	const cfgChannelUUID models.ChannelUUID = "7d3fb8a2-5c1e-4b9f-a6d4-2e8c0f7b5a19"
	const cfgChatID = "Cd4eFg7hIj0kLm3nOp6qRs9t"
	cfgChannel := testdb.InsertChannel(t, rt, testdb.Org1, cfgChannelUUID, map[string]any{"allowed_domains": []string{"example.com"}})
	testdb.InsertVisitor(t, rt, cfgChannel, cfgChatID)

	socket := models.ChatSocket(channelUUID, testChatID)

	presenceTTL := func(socket string) int {
		vc := rt.VK.Get()
		defer vc.Close()
		ttl, err := redis.Int(vc.Do("TTL", centrifugo.SubscriptionKey(socket)))
		require.NoError(t, err)
		return ttl
	}

	// a visitor may subscribe to their own chat: the subscription is recorded as presence on the socket and the
	// realtime server is told when to re-check it
	resp := proxy(s, "subscribe", socket, "")
	assert.Equal(t, 200, resp.Code)
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, resp.Body)
	assert.Equal(t, 150, presenceTTL(socket))

	// and to keep it, with the presence re-armed
	resp = proxy(s, "sub_refresh", socket, "")
	assert.Equal(t, 200, resp.Code)
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, resp.Body)
	assert.Equal(t, 150, presenceTTL(socket))

	// from any site, since the channel isn't pinned to any
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, proxy(s, "subscribe", socket, "https://anywhere.com").Body)

	// everything else is forbidden - in-body, as the proxy protocol requires
	forbidden := func(socket, origin string) {
		t.Helper()

		resp := proxy(s, "subscribe", socket, origin)
		assert.Equal(t, 200, resp.Code, socket)
		assert.JSONEq(t, `{"error": {"code": 403, "message": "forbidden"}}`, resp.Body, socket)

		resp = proxy(s, "sub_refresh", socket, origin)
		assert.Equal(t, 200, resp.Code, socket)
		assert.JSONEq(t, `{"result": {"expired": true}}`, resp.Body, socket)
	}

	forbidden(models.ChatSocket(channelUUID, "Ab1cDe2fGh3iJk4lMn5oPq6r"), "")            // no chat with that chat-id
	forbidden(models.ChatSocket(channelUUID, otherChatID), "")                           // chat-id belongs to a different channel
	forbidden(models.ChatSocket(otherChannelUUID, testChatID), "")                       // and vice versa
	forbidden(models.ChatSocket("11111111-2222-3333-4444-555555555555", testChatID), "") // channel not found

	// malformed sockets never reach a lookup
	forbidden("chat:"+testChatID, "")                                                                      // too few segments
	forbidden(socket+":extra", "")                                                                         // too many segments
	forbidden(models.ChatSocket(channelUUID, testChatID[:23]), "")                                         // chat-id too short
	forbidden(models.ChatSocket(channelUUID, testChatID+"x"), "")                                          // chat-id too long
	forbidden(models.ChatSocket(channelUUID, testChatID[:23]+"-"), "")                                     // chat-id with a non-alphanumeric char
	forbidden(models.ChatSocket(models.ChannelUUID(strings.ToUpper(string(channelUUID))), testChatID), "") // non-canonical uuid encoding
	forbidden(models.ChatSocket("not-a-uuid", testChatID), "")                                             // malformed channel uuid
	forbidden("history:a393abc0-283d-4c9b-a1b3-641a035c34bf", "")                                          // not a chat socket at all
	forbidden("<missing>", "")                                                                             // no socket
	forbidden("<number>", "")                                                                              // malformed payload

	// a pinned channel's chat can be subscribed to from its sites, or from outside a browser...
	cfgSocket := models.ChatSocket(cfgChannelUUID, cfgChatID)
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, proxy(s, "subscribe", cfgSocket, "https://example.com").Body)
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, proxy(s, "subscribe", cfgSocket, "https://EXAMPLE.com").Body)
	assert.JSONEq(t, `{"result": {"expire_at": 1760354490}}`, proxy(s, "subscribe", cfgSocket, "").Body)

	// ...but not from anywhere else
	for _, origin := range []string{"https://evil.com", "https://sub.example.com", "https://example.com:8080", "null"} {
		resp := proxy(s, "subscribe", cfgSocket, origin)
		assert.JSONEq(t, `{"error": {"code": 403, "message": "forbidden"}}`, resp.Body, origin)
		resp = proxy(s, "sub_refresh", cfgSocket, origin)
		assert.JSONEq(t, `{"result": {"expired": true}}`, resp.Body, origin)
	}

	// a released contact's chat can no longer be subscribed to or kept - its presence is left to expire
	testdb.ReleaseContact(t, rt, visitor)
	forbidden(socket, "")
	assert.Equal(t, 150, presenceTTL(socket))

	// nor can a chat on a deactivated channel
	testdb.DeactivateChannel(t, rt, cfgChannel)
	forbidden(cfgSocket, "")
}
