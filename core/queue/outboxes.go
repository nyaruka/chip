package queue

import (
	_ "embed"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/core/models"
)

//go:embed lua/outbox_pop.lua
var outboxPop string
var outboxPopScript = redis.NewScript(2, outboxPop)

type Outboxes struct {
	KeyBase string
}

func (o *Outboxes) AddMessage(rc redis.Conn, msg *models.MsgOut) error {
	rc.Send("MULTI")
	rc.Send("RPUSH", o.keyOutbox(msg.ChatID), jsonx.MustMarshal(msg))   // add message to end of queue
	rc.Send("ZADD", o.keyAll(), "NX", msg.Time.UnixMilli(), msg.ChatID) // update only if it's first message in queue
	_, err := rc.Do("EXEC")
	return err
}

func (o *Outboxes) Boxes(rc redis.Conn) ([]models.ChatID, error) {
	ss, err := redis.Strings(rc.Do("ZRANGE", o.keyAll(), "-inf", "+inf", "BYSCORE"))
	if err != nil {
		return nil, err
	}
	chatIDs := make([]models.ChatID, len(ss))
	for i := range ss {
		chatIDs[i] = models.ChatID(ss[i])
	}
	return chatIDs, nil
}

func (o *Outboxes) PopMessage(rc redis.Conn, chatID models.ChatID) (*models.MsgOut, error) {
	value, err := redis.Bytes(outboxPopScript.Do(rc, o.keyOutbox(chatID), o.keyAll(), chatID))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	msg := &models.MsgOut{}
	jsonx.MustUnmarshal(value, msg)
	return msg, nil
}

func (o *Outboxes) keyOutbox(chatID models.ChatID) string {
	return fmt.Sprintf("%s:queue:%s", o.KeyBase, chatID)
}

func (o *Outboxes) keyAll() string {
	return fmt.Sprintf("%s:queues", o.KeyBase)
}
