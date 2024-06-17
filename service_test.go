package chip_test

import (
	"testing"
	"time"

	"github.com/nyaruka/chip"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
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

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})
	ch, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	require.NoError(t, err)

	client := testsuite.NewClient(t, "ws://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/")

	client.Send(t, `{"type": "start_chat"}`)

	contact, err := models.LoadContact(ctx, rt, orgID, "itlu4O6ZE4ZZc07Y5rHxcLoQ")
	assert.NoError(t, err)
	assert.NotNil(t, contact)

	assert.Equal(t, []string{"StartChat(8291264a-4581-4d12-96e5-e9fcfa6e68d9, itlu4O6ZE4ZZc07Y5rHxcLoQ)"}, mockCourier.Calls)

	// server should send a chat_started event back to the client
	assert.JSONEq(t, `{"type":"chat_started","time":"2024-05-02T16:05:04Z","chat_id":"itlu4O6ZE4ZZc07Y5rHxcLoQ"}`, client.Read(t))

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
		"time": "2024-05-02T16:05:06Z",
		"history": [
			{"type": "msg_in", "time": "2024-05-02T16:05:05Z", "msg_id":1, "text": "hello"}
		]
	}`, client.Read(t))

	// queue a message to be sent to the client
	err = svc.QueueMsgOut(ctx, ch, models.NewMsgOut(123, ch, "itlu4O6ZE4ZZc07Y5rHxcLoQ", "welcome", nil, models.MsgOriginBroadcast, nil, dates.Now()))
	assert.NoError(t, err)

	// and check it is sent to the client
	assert.JSONEq(t, `{"type": "msg_out", "msg_id": 123, "text": "welcome", "origin": "broadcast", "time": "2024-05-02T16:05:07Z"}`, client.Read(t))

	// client acknowledges receipt of the message
	client.Send(t, `{"type": "ack_msg", "msg_id": 123}`)

	client.Close(t)
}
