package queue

import (
	_ "embed"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/gocommon/jsonx"
)

//go:embed lua/outbox_read_ready.lua
var outboxReadReady string
var outboxReadReadyScript = redis.NewScript(2, outboxReadReady)

//go:embed lua/outbox_record_sent.lua
var outboxRecordSent string
var outboxRecordSentScript = redis.NewScript(3, outboxRecordSent)

type Outbox struct {
	KeyBase    string
	InstanceID string
}

func (o *Outbox) SetReady(rc redis.Conn, chatID models.ChatID, ready bool) error {
	var err error
	if ready {
		_, err = rc.Do("SADD", o.readyKey(), chatID)
	} else {
		_, err = rc.Do("SREM", o.readyKey(), chatID)
	}
	return err
}

func (o *Outbox) AddMessage(rc redis.Conn, m *models.MsgOut) error {
	rc.Send("MULTI")
	rc.Send("RPUSH", o.queueKey(m.ChatID), o.encodeMsg(m))
	rc.Send("ZADD", o.queuesKey(), "NX", m.Time.UnixMilli(), m.ChatID) // update only if we're first message in queue
	_, err := rc.Do("EXEC")
	return err
}

func (o *Outbox) ReadReady(rc redis.Conn) ([]*models.MsgOut, error) {
	items, err := redis.ByteSlices(outboxReadReadyScript.Do(rc, o.queuesKey(), o.readyKey(), o.queueKey("")))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	msgs := make([]*models.MsgOut, len(items))
	for i := range items {
		msgs[i] = o.decodeMsg(items[i])
	}

	return msgs, nil
}

func (o *Outbox) RecordSent(rc redis.Conn, chatID models.ChatID, msgID models.MsgID) (bool, error) {
	result, err := redis.Strings(outboxRecordSentScript.Do(rc, o.queuesKey(), o.queueKey(chatID), o.readyKey(), chatID, msgID))
	if err != nil {
		return false, err
	}
	if result[0] == "empty" {
		return false, fmt.Errorf("no messages in queue for chat %s", chatID)
	}
	if result[0] == "wrong-id" {
		return false, fmt.Errorf("expected message id %d in queue, found %s", msgID, result[1])
	}
	return result[1] == "true", nil
}

func (o *Outbox) readyKey() string {
	return fmt.Sprintf("%s:ready:%s", o.KeyBase, o.InstanceID)
}

func (o *Outbox) queuesKey() string {
	return fmt.Sprintf("%s:queues", o.KeyBase)
}

func (o *Outbox) queueKey(chatID models.ChatID) string {
	return fmt.Sprintf("%s:queue:%s", o.KeyBase, chatID)
}

type item struct {
	*models.MsgOut

	TS int64 `json:"_ts"`
}

func (o *Outbox) encodeMsg(m *models.MsgOut) []byte {
	i := &item{MsgOut: m, TS: m.Time.UnixMilli()}
	return jsonx.MustMarshal(i)
}

func (o *Outbox) decodeMsg(b []byte) *models.MsgOut {
	m := &models.MsgOut{}
	jsonx.MustUnmarshal(b, m)
	return m
}
