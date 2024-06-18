package queue_test

import (
	"testing"
	"time"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/core/queue"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/redisx/assertredis"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

func TestOutboxes(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer func() { testsuite.ResetRedis(); testsuite.ResetDB() }()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows", "")
	bob, _ := models.LoadUser(ctx, rt, bobID)
	ch, _ := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")

	o := &queue.Outboxes{KeyBase: "chattest", InstanceID: "foo1"}

	rc := rt.RP.Get()
	defer rc.Close()

	// queue up some messages for 3 chat ids
	err := o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut(101, "hi", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 12, 55, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut(102, "how can I help", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 1, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "3xdF7KhyEiabBiCd3Cst3X28", models.NewMsgOut(103, "hola", nil, models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 32, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut(104, "ok", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 5, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "itlu4O6ZE4ZZc07Y5rHxcLoQ", models.NewMsgOut(105, "test", nil, models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 6, 0, 0, time.UTC)))
	assert.NoError(t, err)

	assertredis.LGetAll(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m101","ts":1706619300000,"msg":{"id":101,"text":"hi","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T12:55:00Z"}}`,
		`{"id":"m102","ts":1706619660000,"msg":{"id":102,"text":"how can I help","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:01:00Z"}}`,
		`{"id":"m104","ts":1706619900000,"msg":{"id":104,"text":"ok","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:05:00Z"}}`,
	})
	assertredis.LGetAll(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m103","ts":1706621520000,"msg":{"id":103,"text":"hola","origin":"flow","time":"2024-01-30T13:32:00Z"}}`,
	})
	assertredis.LGetAll(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m105","ts":1706619960000,"msg":{"id":105,"text":"test","origin":"flow","time":"2024-01-30T13:06:00Z"}}`,
	})
	assertredis.ZGetAll(t, rc, "chattest:outboxes", map[string]float64{
		"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619300000,
		"3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706621520000,
		"itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619960000,
	})

	// currently no chat ids are marked ready, so reading messages should give us nothing
	ready, err := o.ReadReady(rc)
	assert.NoError(t, err)
	assert.Len(t, ready, 0)

	// mark 2 chat ids as ready
	err = o.SetReady(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", true)
	assert.NoError(t, err)
	err = o.SetReady(rc, ch, "itlu4O6ZE4ZZc07Y5rHxcLoQ", true)
	assert.NoError(t, err)
	assertredis.SMembers(t, rc, "chattest:ready:foo1", []string{"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9"})

	// reading should now give us their oldest messages
	ready, err = o.ReadReady(rc)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []queue.Outbox{{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "65vbbDAQCdPdEWlEhDGy4utO"}, {"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ"}}, maps.Keys(ready))
	assert.Equal(t, queue.ItemID("m101"), ready[queue.Outbox{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "65vbbDAQCdPdEWlEhDGy4utO"}].ID)
	assert.Equal(t, queue.ItemID("m105"), ready[queue.Outbox{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ"}].ID)

	// and remove them from the instance's ready set
	assertredis.SMembers(t, rc, "chattest:ready:foo1", []string{})

	// nothing actual removed from any of the outboxes
	assertredis.LLen(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 3)
	assertredis.LLen(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)
	assertredis.LLen(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)

	hasMore, err := o.RecordSent(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", "m101")
	assert.NoError(t, err)
	assert.True(t, hasMore)

	// msg should be removed from the outbox for that chat, other chat outboxes should be unchanged
	assertredis.LGetAll(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m102","ts":1706619660000,"msg":{"id":102,"text":"how can I help","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:01:00Z"}}`,
		`{"id":"m104","ts":1706619900000,"msg":{"id":104,"text":"ok","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:05:00Z"}}`,
	})
	assertredis.LLen(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)
	assertredis.LLen(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)

	assertredis.ZGetAll(t, rc, "chattest:outboxes", map[string]float64{
		"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619660000, // updated to new oldest message
		"3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706621520000,
		"itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619960000,
	})

	// and outbox should be back in the ready set
	assertredis.SMembers(t, rc, "chattest:ready:foo1", []string{"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9"})

	// try recording sent for a chat with an empty outbox
	_, err = o.RecordSent(rc, ch, "A0UGLTWLLs59CrFzj6VpvMlG", "m101")
	assert.EqualError(t, err, "outbox empty for chat A0UGLTWLLs59CrFzj6VpvMlG")

	// try recording sent with an incorrect message ID
	_, err = o.RecordSent(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", "m999")
	assert.EqualError(t, err, "expected item id m999 in outbox, found m102")
}
