package web_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nyaruka/chip/core/courier"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/chip/web"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockService struct {
	store   models.Store
	courier courier.Courier
}

func (s *MockService) Store() models.Store                           { return s.store }
func (s *MockService) Courier() courier.Courier                      { return s.courier }
func (s *MockService) OnChatStarted(*models.Channel, models.ChatID)  {}
func (s *MockService) OnChatClosed(*models.Channel, models.ChatID)   {}
func (s *MockService) OnSendRequest(*models.Channel, *models.MsgOut) {}

func TestServer(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	defer random.SetGenerator(random.DefaultGenerator)
	random.SetGenerator(random.NewSeededGenerator(1234))

	defer dates.SetNowSource(dates.DefaultNowSource)
	dates.SetNowSource(dates.NewSequentialNowSource(time.Date(2024, 5, 2, 16, 5, 4, 0, time.UTC)))

	mockCourier := testsuite.NewMockCourier(rt)
	mockSvc := &MockService{store: models.NewStore(rt), courier: mockCourier}

	server := web.NewServer(rt, mockSvc)
	server.Start()
	defer server.Stop()

	req, _ := http.NewRequest("GET", "http://localhost:8071/", nil)
	trace, err := httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, trace.Response.StatusCode)
	assert.Equal(t, `{"version":"Dev"}`, string(trace.ResponseBody))

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})

	// try to start for a non-existent channel
	req, _ = http.NewRequest("POST", "http://localhost:8071/wc/connect/16955bac-23fd-4b5f-8981-530679ae0ac4/", nil)
	trace, err = httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, `{"error":"no such channel"}`, string(trace.ResponseBody))

	// try to start against an existing channel (still fails because HTTP client does support web sockets)
	req, _ = http.NewRequest("POST", "http://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/", nil)
	trace, err = httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, "Bad Request\n", string(trace.ResponseBody))

	c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/", nil)
	require.NoError(t, err)
	require.NotNil(t, c)

	send := func(m string) {
		err := c.WriteMessage(websocket.TextMessage, []byte(m))
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	}
	read := func() string {
		_, d, err := c.ReadMessage()
		assert.NoError(t, err)
		return string(d)
	}

	send(`{"type": "start_chat"}`)

	contact, err := models.LoadContact(ctx, rt, orgID, "itlu4O6ZE4ZZc07Y5rHxcLoQ")
	assert.NoError(t, err)
	assert.NotNil(t, contact)

	assert.Equal(t, []string{"StartChat(8291264a-4581-4d12-96e5-e9fcfa6e68d9, itlu4O6ZE4ZZc07Y5rHxcLoQ)"}, mockCourier.Calls)

	// server should send a chat_started event back to the client
	assert.JSONEq(t, `{"type":"chat_started","time":"2024-05-02T16:05:10Z","chat_id":"itlu4O6ZE4ZZc07Y5rHxcLoQ"}`, read())

	send(`{"type": "send_msg", "text": "hello"}`)

	assert.Equal(t, []string{
		"StartChat(8291264a-4581-4d12-96e5-e9fcfa6e68d9, itlu4O6ZE4ZZc07Y5rHxcLoQ)",
		"CreateMsg(8291264a-4581-4d12-96e5-e9fcfa6e68d9, 1, 'hello')",
	}, mockCourier.Calls)

	send(`{"type": "set_email", "email": "bob@nyaruka.com"}`)

	// reload contact and check email is now set
	contact, err = models.LoadContact(ctx, rt, orgID, "itlu4O6ZE4ZZc07Y5rHxcLoQ")
	assert.NoError(t, err)
	assert.Equal(t, "bob@nyaruka.com", contact.Email)

	send(`{"type": "get_history", "before": "2024-05-02T16:05:12Z"}`)

	// server should send a history event back to the client
	assert.JSONEq(t, `{
		"type": "history",
		"time": "2024-05-02T16:05:12Z",
		"history": [
			{"type": "msg_in", "time": "2024-05-02T16:05:11Z", "msg_id":1, "text": "hello"}
		]
	}`, read())

	c.Close()

	time.Sleep(100 * time.Millisecond)
}
