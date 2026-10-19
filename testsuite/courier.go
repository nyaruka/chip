package testsuite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/uuids"
)

// CourierRequest is a request the mock courier received
type CourierRequest struct {
	Action      string          `json:"action"`
	ChannelUUID string          `json:"channel_uuid"`
	Auth        string          `json:"auth"`
	Body        json.RawMessage `json:"body"`
}

// MockCourier stands in for courier at the HTTP level, so that chip's real client code is exercised. By default it
// accepts everything the way courier does; a test can script a different answer for an action.
type MockCourier struct {
	server *httptest.Server

	mu        sync.Mutex
	requests  []*CourierRequest
	responses map[string]*mockResponse
}

type mockResponse struct {
	status int
	body   string
}

// NewMockCourier starts a mock courier and points the runtime's config at it
func NewMockCourier(t *testing.T, rt *runtime.Runtime) *MockCourier {
	m := &MockCourier{responses: map[string]*mockResponse{}}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.server.Close)

	rt.Config.CourierURL = m.server.URL
	return m
}

// Respond scripts the answer to the given action from now on, in place of the default acceptance
func (m *MockCourier) Respond(action string, status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses[action] = &mockResponse{status, body}
}

// Requests returns the requests received so far, oldest first
func (m *MockCourier) Requests() []*CourierRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]*CourierRequest(nil), m.requests...)
}

// Clear forgets the requests received so far
func (m *MockCourier) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = nil
}

func (m *MockCourier) handle(w http.ResponseWriter, r *http.Request) {
	// requests are to /c/wch/<channel-uuid>/<action>
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "c" || parts[1] != "wch" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	channelUUID, action := parts[2], parts[3]

	body, _ := io.ReadAll(r.Body)

	m.mu.Lock()
	m.requests = append(m.requests, &CourierRequest{Action: action, ChannelUUID: channelUUID, Auth: r.Header.Get("Authorization"), Body: body})
	resp := m.responses[action]
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if resp != nil {
		w.WriteHeader(resp.status)
		w.Write([]byte(resp.body))
		return
	}

	// courier's own shapes for accepting each kind of request
	switch action {
	case "start":
		w.Write([]byte(`{"message": "Contact Created", "data": []}`))
	case "receive":
		w.Write([]byte(fmt.Sprintf(`{"message": "Message Accepted", "data": [{"type": "msg", "msg_uuid": "%s"}]}`, uuids.NewV7())))
	case "status":
		w.Write([]byte(`{"message": "Status Update Accepted", "data": [{"type": "status"}]}`))
	default:
		http.NotFound(w, r)
	}
}
