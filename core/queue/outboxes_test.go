package queue_test

import (
	"testing"
	"time"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/core/queue"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/vkutil/assertvk"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

func TestOutboxes(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer func() { testsuite.ResetValkey(); testsuite.ResetDB() }()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"}, map[string]any{"secret": "sesame"})
	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows", "")
	bob, _ := models.LoadUser(ctx, rt, bobID)
	ch, _ := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")

	o := &queue.Outboxes{KeyBase: "chattest", InstanceID: "foo1"}

	rc := rt.RP.Get()
	defer rc.Close()

	// queue up some messages for 3 chat ids
	err := o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut("019a0719-ac96-7213-92ec-172fd22ef691", "hi", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 12, 55, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut("019a0719-ac96-70d7-a177-0cc863fdff13", "how can I help", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 1, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "3xdF7KhyEiabBiCd3Cst3X28", models.NewMsgOut("019a0719-ac96-7aba-b890-b25232e1ae9f", "hola", nil, models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 32, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", models.NewMsgOut("019a0719-ac96-7077-bec7-90f04f673481", "ok", nil, models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 5, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, ch, "itlu4O6ZE4ZZc07Y5rHxcLoQ", models.NewMsgOut("019a0719-ac96-714d-9217-ba45032ce93f", "test", nil, models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 6, 0, 0, time.UTC)))
	assert.NoError(t, err)

	assertvk.LGetAll(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m019a0719-ac96-7213-92ec-172fd22ef691","ts":1706619300000,"msg":{"uuid":"019a0719-ac96-7213-92ec-172fd22ef691","text":"hi","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T12:55:00Z"}}`,
		`{"id":"m019a0719-ac96-70d7-a177-0cc863fdff13","ts":1706619660000,"msg":{"uuid":"019a0719-ac96-70d7-a177-0cc863fdff13","text":"how can I help","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:01:00Z"}}`,
		`{"id":"m019a0719-ac96-7077-bec7-90f04f673481","ts":1706619900000,"msg":{"uuid":"019a0719-ac96-7077-bec7-90f04f673481","text":"ok","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:05:00Z"}}`,
	})
	assertvk.LGetAll(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m019a0719-ac96-7aba-b890-b25232e1ae9f","ts":1706621520000,"msg":{"uuid":"019a0719-ac96-7aba-b890-b25232e1ae9f","text":"hola","origin":"flow","time":"2024-01-30T13:32:00Z"}}`,
	})
	assertvk.LGetAll(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m019a0719-ac96-714d-9217-ba45032ce93f","ts":1706619960000,"msg":{"uuid":"019a0719-ac96-714d-9217-ba45032ce93f","text":"test","origin":"flow","time":"2024-01-30T13:06:00Z"}}`,
	})
	assertvk.ZGetAll(t, rc, "chattest:outboxes", map[string]float64{
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
	assertvk.SMembers(t, rc, "chattest:ready:foo1", []string{"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9"})

	// reading should now give us their oldest messages
	ready, err = o.ReadReady(rc)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []queue.Outbox{{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "65vbbDAQCdPdEWlEhDGy4utO"}, {"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ"}}, maps.Keys(ready))
	assert.Equal(t, queue.ItemID("m019a0719-ac96-7213-92ec-172fd22ef691"), ready[queue.Outbox{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "65vbbDAQCdPdEWlEhDGy4utO"}].ID)
	assert.Equal(t, queue.ItemID("m019a0719-ac96-714d-9217-ba45032ce93f"), ready[queue.Outbox{"8291264a-4581-4d12-96e5-e9fcfa6e68d9", "itlu4O6ZE4ZZc07Y5rHxcLoQ"}].ID)

	// and remove them from the instance's ready set
	assertvk.SMembers(t, rc, "chattest:ready:foo1", []string{})

	// nothing actual removed from any of the outboxes
	assertvk.LLen(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 3)
	assertvk.LLen(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)
	assertvk.LLen(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)

	hasMore, err := o.RecordSent(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", "m019a0719-ac96-7213-92ec-172fd22ef691")
	assert.NoError(t, err)
	assert.True(t, hasMore)

	// msg should be removed from the outbox for that chat, other chat outboxes should be unchanged
	assertvk.LGetAll(t, rc, "chattest:outbox:65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9", []string{
		`{"id":"m019a0719-ac96-70d7-a177-0cc863fdff13","ts":1706619660000,"msg":{"uuid":"019a0719-ac96-70d7-a177-0cc863fdff13","text":"how can I help","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:01:00Z"}}`,
		`{"id":"m019a0719-ac96-7077-bec7-90f04f673481","ts":1706619900000,"msg":{"uuid":"019a0719-ac96-7077-bec7-90f04f673481","text":"ok","origin":"chat","user":{"id":1,"email":"bob@nyaruka.com","name":"Bob McFlows"},"time":"2024-01-30T13:05:00Z"}}`,
	})
	assertvk.LLen(t, rc, "chattest:outbox:3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)
	assertvk.LLen(t, rc, "chattest:outbox:itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9", 1)

	assertvk.ZGetAll(t, rc, "chattest:outboxes", map[string]float64{
		"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619660000, // updated to new oldest message
		"3xdF7KhyEiabBiCd3Cst3X28@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706621520000,
		"itlu4O6ZE4ZZc07Y5rHxcLoQ@8291264a-4581-4d12-96e5-e9fcfa6e68d9": 1706619960000,
	})

	// and outbox should be back in the ready set
	assertvk.SMembers(t, rc, "chattest:ready:foo1", []string{"65vbbDAQCdPdEWlEhDGy4utO@8291264a-4581-4d12-96e5-e9fcfa6e68d9"})

	// try recording sent for a chat with an empty outbox
	_, err = o.RecordSent(rc, ch, "A0UGLTWLLs59CrFzj6VpvMlG", "m019a0719-ac96-7213-92ec-172fd22ef691")
	assert.EqualError(t, err, "outbox empty for chat A0UGLTWLLs59CrFzj6VpvMlG")

	// try recording sent with an incorrect message ID
	_, err = o.RecordSent(rc, ch, "65vbbDAQCdPdEWlEhDGy4utO", "m019a0719-ac96-737b-83ce-1e742e468575")
	assert.EqualError(t, err, "expected item id m019a0719-ac96-737b-83ce-1e742e468575 in outbox, found m019a0719-ac96-70d7-a177-0cc863fdff13")
}
