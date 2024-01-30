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

	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows")
	bob, _ := models.LoadUser(ctx, rt, bobID)

	o := &queue.Outboxes{KeyBase: "chattest"}

	rc := rt.RP.Get()
	defer rc.Close()

	err := o.AddMessage(rc, models.NewMsgOut(101, "65vbbDAQCdPdEWlEhDGy4utO", "hi", models.MsgOriginChat, bob, time.Date(2024, 1, 30, 12, 55, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, models.NewMsgOut(102, "65vbbDAQCdPdEWlEhDGy4utO", "how can I help", models.MsgOriginChat, bob, time.Date(2024, 1, 30, 13, 1, 0, 0, time.UTC)))
	assert.NoError(t, err)
	err = o.AddMessage(rc, models.NewMsgOut(103, "3xdF7KhyEiabBiCd3Cst3X28", "hola", models.MsgOriginFlow, nil, time.Date(2024, 1, 30, 13, 32, 0, 0, time.UTC)))
	assert.NoError(t, err)

	assertredis.LLen(t, rt.RP, "chattest:queue:65vbbDAQCdPdEWlEhDGy4utO", 2)
	assertredis.LRange(t, rt.RP, "chattest:queue:65vbbDAQCdPdEWlEhDGy4utO", 0, 2, []string{
		fmt.Sprintf(`{"id":101,"chat_id":"65vbbDAQCdPdEWlEhDGy4utO","text":"hi","origin":"chat","user_id":%d,"time":"2024-01-30T12:55:00Z"}`, bob.ID()),
		fmt.Sprintf(`{"id":102,"chat_id":"65vbbDAQCdPdEWlEhDGy4utO","text":"how can I help","origin":"chat","user_id":%d,"time":"2024-01-30T13:01:00Z"}`, bob.ID()),
	})
	assertredis.LLen(t, rt.RP, "chattest:queue:3xdF7KhyEiabBiCd3Cst3X28", 1)
	assertredis.ZCard(t, rt.RP, "chattest:queues", 2)
	assertredis.ZScore(t, rt.RP, "chattest:queues", "65vbbDAQCdPdEWlEhDGy4utO", 1706619300000)
	assertredis.ZScore(t, rt.RP, "chattest:queues", "3xdF7KhyEiabBiCd3Cst3X28", 1706621520000)

	boxes, err := o.Boxes(rc)
	assert.NoError(t, err)
	assert.Equal(t, []models.ChatID{"65vbbDAQCdPdEWlEhDGy4utO", "3xdF7KhyEiabBiCd3Cst3X28"}, boxes)

	msg, err := o.PopMessage(rc, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, models.MsgID(101), msg.ID)
	assert.Equal(t, "hi", msg.Text)
	assertredis.LLen(t, rt.RP, "chattest:queue:65vbbDAQCdPdEWlEhDGy4utO", 1)
	assertredis.ZCard(t, rt.RP, "chattest:queues", 2)
	assertredis.ZScore(t, rt.RP, "chattest:queues", "65vbbDAQCdPdEWlEhDGy4utO", 1706619300000)

	msg, err = o.PopMessage(rc, "3xdF7KhyEiabBiCd3Cst3X28")
	assert.NoError(t, err)
	assert.Equal(t, models.MsgID(103), msg.ID)
	assert.Equal(t, "hola", msg.Text)
	assertredis.LLen(t, rt.RP, "chattest:queue:3xdF7KhyEiabBiCd3Cst3X28", 0)
	assertredis.ZCard(t, rt.RP, "chattest:queues", 1)
}
