package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nyaruka/chip/v26/core/courier"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/chip/v26/testsuite"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/nyaruka/gocommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	channelUUID      models.ChannelUUID = "0665bf36-4d2e-4c3f-b8a1-9f8e6a5c2d71"
	otherChannelUUID models.ChannelUUID = "b81c3f45-2d6e-4a1f-9c72-8e5d0a4b6f13"

	// the users in the dump
	adminEmail = "admin1@textit.com" // Andy Admin
	agentEmail = "agent1@textit.com" // Ann D'Agent
	adminUUID  = "ad9fdf9f-56ab-422a-b77d-e3ec26091a25"
	agentUUID  = "aee44553-ab35-482b-a4d4-6f000ec611ab"

	testChatID = "vM0GGhDrqpTQefIEinK0up3C" // what the secure source seeded below mints first
)

// setup returns a runtime, a server on it and the mock courier the server talks to, with the channel inserted and
// chat ID minting made deterministic
func setup(t *testing.T) (*runtime.Runtime, *Server, *testsuite.MockCourier) {
	_, rt := testsuite.Runtime(t)
	mock := testsuite.NewMockCourier(t, rt)

	random.SetSecureSource(random.NewSeededSource(1234))
	t.Cleanup(func() { random.SetSecureSource(random.DefaultSecureSource) })

	testdb.InsertChannel(t, rt, testdb.Org1, channelUUID, nil)

	return rt, NewServer(rt, courier.NewClient(rt)), mock
}

type response struct {
	Code   int
	Header http.Header
	Body   string
}

// JSON unmarshals the response body
func (r *response) JSON(t *testing.T, v any) {
	require.NoError(t, json.Unmarshal([]byte(r.Body), v), "body not JSON: %s", r.Body)
}

// request makes a request of the given handler
func request(h http.Handler, req *http.Request) *response {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Result().Body)
	return &response{Code: rr.Code, Header: rr.Header(), Body: string(body)}
}

// public makes a request of a channel's endpoint on the public listener, from the given IP and origin
func public(s *Server, method string, ch models.ChannelUUID, action, query, body, ip, origin string) *response {
	req := httptest.NewRequest(method, "https://localhost"+publicPrefix+"/"+string(ch)+"/"+action+query, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.RemoteAddr = ip
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return request(s.PublicHandler(), req)
}

// post posts JSON to a channel's endpoint
func post(s *Server, ch models.ChannelUUID, action, body string) *response {
	return public(s, http.MethodPost, ch, action, "", body, "", "")
}

// internal posts JSON to the internal listener with the given auth header
func internal(s *Server, path, auth, body string) *response {
	req := httptest.NewRequest(http.MethodPost, "https://localhost"+internalPrefix+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return request(s.InternalHandler(), req)
}

// start starts a chat and returns the chat ID
func start(t *testing.T, s *Server) string {
	resp := post(s, channelUUID, "start", `{}`)
	require.Equal(t, 200, resp.Code, resp.Body)

	r := &startResponse{}
	resp.JSON(t, r)
	return r.ChatID
}

func TestHealth(t *testing.T) {
	_, s, _ := setup(t)

	resp := request(s.PublicHandler(), httptest.NewRequest(http.MethodGet, "https://localhost/", nil))
	assert.Equal(t, 200, resp.Code)
	assert.JSONEq(t, `{"component": "chip", "listener": "public", "version": "Dev"}`, resp.Body)

	resp = request(s.InternalHandler(), httptest.NewRequest(http.MethodGet, "https://localhost/", nil))
	assert.Equal(t, 200, resp.Code)
	assert.JSONEq(t, `{"component": "chip", "listener": "internal", "version": "Dev"}`, resp.Body)
}

func TestChannelRoute(t *testing.T) {
	rt, s, _ := setup(t)

	// a channel that doesn't exist, isn't a webchat channel or isn't active is not found
	resp := post(s, "11111111-2222-3333-4444-555555555555", "start", `{}`)
	assert.Equal(t, 404, resp.Code)
	assert.JSONEq(t, `{"error": "channel not found"}`, resp.Body)

	resp = post(s, "not-a-uuid", "start", `{}`)
	assert.Equal(t, 404, resp.Code)

	ch := testdb.InsertChannel(t, rt, testdb.Org1, otherChannelUUID, nil)
	assert.Equal(t, 200, post(s, otherChannelUUID, "start", `{}`).Code)

	testdb.DeactivateChannel(t, rt, ch)
	assert.Equal(t, 404, post(s, otherChannelUUID, "start", `{}`).Code)
}

// the internal listener is only for our own services
func TestInternalAuth(t *testing.T) {
	rt, s, _ := setup(t)

	body := `{"channel_uuid": "` + string(channelUUID) + `", "chat_id": "` + testChatID + `", "type": "typing_started"}`

	for _, auth := range []string{"", "Token wrong", "Bearer sesame", "sesame"} {
		resp := internal(s, "/event", auth, body)
		assert.Equal(t, 401, resp.Code, auth)
		assert.JSONEq(t, `{"error": "invalid or missing authorization header"}`, resp.Body)
	}

	assert.Equal(t, 200, internal(s, "/event", "Token sesame", body).Code)

	// unless no token is configured, as in a dev environment
	rt.Config.AuthToken = ""
	assert.Equal(t, 200, internal(s, "/event", "", body).Code)
}
