package web_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/nyaruka/chip"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()
	defer testsuite.ResetRedis()

	defer random.SetGenerator(random.DefaultGenerator)
	random.SetGenerator(random.NewSeededGenerator(1234))

	defer dates.SetNowSource(dates.DefaultNowSource)
	dates.SetNowSource(dates.NewSequentialNowSource(time.Date(2024, 5, 2, 16, 5, 4, 0, time.UTC)))

	mockCourier := testsuite.NewMockCourier(rt)

	svc := chip.NewService(rt, mockCourier)
	assert.NoError(t, svc.Start())

	defer svc.Stop()

	req, _ := http.NewRequest("GET", "http://localhost:8071/", nil)
	trace, err := httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, trace.Response.StatusCode)
	assert.Equal(t, `{"version":"Dev"}`, string(trace.ResponseBody))

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"}, map[string]any{"secret": "sesame"})
	ch, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	require.NoError(t, err)

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

	client := testsuite.NewClient(t, "ws://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/")

	client.Send(t, `{"type": "start_chat"}`)

	contact, err := models.LoadContact(ctx, rt, orgID, "itlu4O6ZE4ZZc07Y5rHxcLoQ")
	assert.NoError(t, err)
	assert.NotNil(t, contact)

	assert.Equal(t, []string{"StartChat(8291264a-4581-4d12-96e5-e9fcfa6e68d9, itlu4O6ZE4ZZc07Y5rHxcLoQ)"}, mockCourier.Calls)

	// server should send a chat_started event back to the client
	assert.JSONEq(t, `{"type":"chat_started","chat_id":"itlu4O6ZE4ZZc07Y5rHxcLoQ"}`, client.Read(t))

	client.Send(t, `{"type": "send_msg", "text": "hello"}`)

	assert.Equal(t, []string{
		"StartChat(8291264a-4581-4d12-96e5-e9fcfa6e68d9, itlu4O6ZE4ZZc07Y5rHxcLoQ)",
		"CreateMsg(8291264a-4581-4d12-96e5-e9fcfa6e68d9, 1, 'hello')",
	}, mockCourier.Calls)

	client.Send(t, `{"type": "set_email", "email": "bob@nyaruka.com"}`)

	// reload contact and check email is now set
	contact, err = models.LoadContact(ctx, rt, orgID, "itlu4O6ZE4ZZc07Y5rHxcLoQ")
	assert.NoError(t, err)
	assert.Equal(t, "bob@nyaruka.com", contact.Email)

	client.Send(t, `{"type": "get_history", "before": "2024-05-02T16:05:12Z"}`)

	// server should send a history event back to the client
	assert.JSONEq(t, `{
		"type": "history",
		"history": [
			{"msg_in": {"id":1, "text": "hello", "time": "2024-05-02T16:05:10Z"}}
		]
	}`, client.Read(t))

	// queue a message to be sent to the client
	err = svc.QueueMsgOut(ctx, ch, contact, models.NewMsgOut(123, "welcome", nil, models.MsgOriginBroadcast, nil, dates.Now()))
	assert.NoError(t, err)

	// and check it is sent to the client
	assert.JSONEq(t, `{"type": "chat_out", "msg_out": {"id": 123, "text": "welcome", "origin": "broadcast", "time": "2024-05-02T16:05:11Z"}}`, client.Read(t))

	// client acknowledges receipt of the message
	client.Send(t, `{"type": "ack_chat", "msg_id": 123}`)

	assert.Equal(t, "ReportDelivered(8291264a-4581-4d12-96e5-e9fcfa6e68d9, 1, 123)", mockCourier.Calls[2])

	client.Close(t)
	time.Sleep(100 * time.Millisecond)
}
