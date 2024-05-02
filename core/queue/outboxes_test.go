package queue_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nyaruka/redisx/assertredis"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/core/queue"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestOutboxes(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer func() { testsuite.ResetRedis(); testsuite.ResetDB() }()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows")

	bob, _ := models.LoadUser(ctx, rt, bobID)
	ch, _ := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")

	o := &queue.Outboxes{KeyBase: "chattest"}

	rc := rt.RP.Get()
	defer rc.Close()

	err := o.AddMessage(rc, ch, models.NewMsgOut(101, "65vbbDAQCdPdEWlEhDGy4utO", "hi", models.MsgOriginChat, bob, time.Date(2024, 1, 30, 12, 55, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, models.NewMsgOut(102, "65vbbDAQCdPdEWlEhDGy4utO", "how can I help", models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 1, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, models.NewMsgOut(103, "3xdF7KhyEiabBiCd3Cst3X28", "hola", models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 32, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, models.NewMsgOut(104, "65vbbDAQCdPdEWlEhDGy4utO", "ok", models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 5, 0, 0, time.UTC)))
	assert.NoError(t, err)

	assertredis.LLen(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 3)
	assertredis.LRange(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 0, 2, []string{
		fmt.Sprintf(`1706619300000|{"id":101,"chat_id":"65vbbDAQCdPdEWlEhDGy4utO","text":"hi","origin":"chat","user_id":%d,"time":"2024-01-30T12:55:00Z"}`, bob.ID),
		fmt.Sprintf(`1706619660000|{"id":102,"chat_id":"65vbbDAQCdPdEWlEhDGy4utO","text":"how can I help","origin":"chat","user_id":%d,"time":"2024-01-30T13:01:00Z"}`, bob.ID),
		fmt.Sprintf(`1706619900000|{"id":104,"chat_id":"65vbbDAQCdPdEWlEhDGy4utO","text":"ok","origin":"chat","user_id":%d,"time":"2024-01-30T13:05:00Z"}`, bob.ID),
	})
	assertredis.LLen(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:3xdF7KhyEiabBiCd3Cst3X28", 1)
	assertredis.ZCard(t, rc, "chattest:queues", 2)
	assertredis.ZScore(t, rc, "chattest:queues", "8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 1706619300000)
	assertredis.ZScore(t, rc, "chattest:queues", "8291264a-4581-4d12-96e5-e9fcfa6e68d9:3xdF7KhyEiabBiCd3Cst3X28", 1706621520000)

	boxes, err := o.All(rc)
	assert.NoError(t, err)
	assert.Equal(t, []*queue.Outbox{
		{ChannelUUID: "8291264a-4581-4d12-96e5-e9fcfa6e68d9", ChatID: "65vbbDAQCdPdEWlEhDGy4utO", Oldest: time.Date(2024, time.January, 30, 12, 55, 0, 0, time.UTC)},
		{ChannelUUID: "8291264a-4581-4d12-96e5-e9fcfa6e68d9", ChatID: "3xdF7KhyEiabBiCd3Cst3X28", Oldest: time.Date(2024, time.January, 30, 13, 32, 0, 0, time.UTC)},
	}, boxes)

	msg, err := o.PopMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.MsgID(101), msg.ID)
	assert.Equal(t, "hi", msg.Text)
	assertredis.LLen(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 2)
	assertredis.ZCard(t, rc, "chattest:queues", 2)
	assertredis.ZScore(t, rc, "chattest:queues", "8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 1706619660000)

	msgs, err := o.PopAll(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO") // pops remaining 2 messages
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	assert.Equal(t, models.MsgID(102), msgs[0].ID)
	assert.Equal(t, models.MsgID(104), msgs[1].ID)
	assertredis.LLen(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:65vbbDAQCdPdEWlEhDGy4utO", 0)
	assertredis.ZCard(t, rc, "chattest:queues", 1)

	msg, err = o.PopMessage(rc, ch, "3xdF7KhyEiabBiCd3Cst3X28") // last and only message
	assert.NoError(t, err)
	assert.Equal(t, models.MsgID(103), msg.ID)
	assert.Equal(t, "hola", msg.Text)
	assertredis.LLen(t, rc, "chattest:queue:8291264a-4581-4d12-96e5-e9fcfa6e68d9:3xdF7KhyEiabBiCd3Cst3X28", 0)
	assertredis.ZCard(t, rc, "chattest:queues", 0)
}
