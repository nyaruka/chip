package queue

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/gocommon/jsonx"
)

//go:embed lua/outboxes_read_ready.lua
var outboxesReadReady string
var outboxesReadReadyScript = redis.NewScript(2, outboxesReadReady)

//go:embed lua/outboxes_record_sent.lua
var outboxesRecordSent string
var outboxesRecordSentScript = redis.NewScript(3, outboxesRecordSent)

type Outbox struct {
	ChannelUUID models.ChannelUUID
	ChatID      models.ChatID
}

func (q Outbox) String() string {
	return fmt.Sprintf("%s@%s", q.ChatID, q.ChannelUUID)
}

type Outboxes struct {
	KeyBase    string
	InstanceID string
}

func (o *Outboxes) SetReady(rc redis.Conn, ch *models.Channel, chatID models.ChatID, ready bool) error {
	queue := Outbox{ch.UUID, chatID}

	var err error
	if ready {
		_, err = rc.Do("SADD", o.readyKey(), queue.String())
	} else {
		_, err = rc.Do("SREM", o.readyKey(), queue.String())
	}
	return err
}

func (o *Outboxes) AddMessage(rc redis.Conn, ch *models.Channel, chatID models.ChatID, m *models.MsgOut) error {
	queue := Outbox{ch.UUID, chatID}

	rc.Send("MULTI")
	rc.Send("RPUSH", o.queueKey(queue), encodeMsg(m))
	rc.Send("ZADD", o.queuesKey(), "NX", m.Time.UnixMilli(), queue.String()) // update only if we're first message in queue
	_, err := rc.Do("EXEC")
	return err
}

func (o *Outboxes) ReadReady(rc redis.Conn) (map[Outbox]*models.MsgOut, error) {
	pairs, err := redis.ByteSlices(outboxesReadReadyScript.Do(rc, o.queuesKey(), o.readyKey(), o.KeyBase))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	ready := make(map[Outbox]*models.MsgOut, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		queue := string(pairs[i])
		item := pairs[i+1]
		ready[decodeQueue(queue)] = decodeMsg(item)
	}

	return ready, nil
}

func (o *Outboxes) RecordSent(rc redis.Conn, ch *models.Channel, chatID models.ChatID, msgID models.MsgID) (bool, error) {
	queue := Outbox{ch.UUID, chatID}

	result, err := redis.Strings(outboxesRecordSentScript.Do(rc, o.queuesKey(), o.queueKey(queue), o.readyKey(), queue.String(), msgID))
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

func (o *Outboxes) readyKey() string {
	return fmt.Sprintf("%s:ready:%s", o.KeyBase, o.InstanceID)
}

func (o *Outboxes) queuesKey() string {
	return fmt.Sprintf("%s:queues", o.KeyBase)
}

func (o *Outboxes) queueKey(queue Outbox) string {
	return fmt.Sprintf("%s:queue:%s", o.KeyBase, queue)
}

type item struct {
	*models.MsgOut

	TS int64 `json:"_ts"`
}

func encodeMsg(m *models.MsgOut) []byte {
	i := &item{MsgOut: m, TS: m.Time.UnixMilli()}
	return jsonx.MustMarshal(i)
}

func decodeMsg(b []byte) *models.MsgOut {
	m := &models.MsgOut{}
	jsonx.MustUnmarshal(b, m)
	return m
}

func decodeQueue(q string) Outbox {
	parts := strings.Split(q, "@")
	return Outbox{models.ChannelUUID(parts[1]), models.ChatID(parts[0])}
}
