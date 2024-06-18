package queue

import (
	_ "embed"
	"encoding/json"
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

type ItemID string

// Item wraps things that can be put in an outbox
type Item struct {
	ID  ItemID         `json:"id"`
	TS  int64          `json:"ts"`
	Msg *models.MsgOut `json:"msg"`
}

// Outbox is channel + chat ID pair that we can send to
type Outbox struct {
	ChannelUUID models.ChannelUUID
	ChatID      models.ChatID
}

func (q Outbox) String() string {
	return fmt.Sprintf("%s@%s", q.ChatID, q.ChannelUUID)
}

func decodeOutbox(id string) Outbox {
	parts := strings.Split(id, "@")
	return Outbox{models.ChannelUUID(parts[1]), models.ChatID(parts[0])}
}

type Outboxes struct {
	KeyBase    string
	InstanceID string
}

// SetReady records that this instance is ready to send messages to the given chat id
func (o *Outboxes) SetReady(rc redis.Conn, ch *models.Channel, chatID models.ChatID, ready bool) error {
	outbox := Outbox{ch.UUID, chatID}

	var err error
	if ready {
		_, err = rc.Do("SADD", o.readyKey(), outbox.String())
	} else {
		_, err = rc.Do("SREM", o.readyKey(), outbox.String())
	}
	return err
}

// AddMessage adds a message to the outbox for the given chat id
func (o *Outboxes) AddMessage(rc redis.Conn, ch *models.Channel, chatID models.ChatID, m *models.MsgOut) error {
	outbox := Outbox{ch.UUID, chatID}
	item := &Item{ID: ItemID(fmt.Sprintf("m%d", m.ID)), TS: m.Time.UnixMilli(), Msg: m}

	rc.Send("MULTI")
	rc.Send("RPUSH", o.outboxKey(outbox), jsonx.MustMarshal(item))
	rc.Send("ZADD", o.allKey(), "NX", m.Time.UnixMilli(), outbox.String()) // update only if we're first message
	_, err := rc.Do("EXEC")
	return err
}

// ReadReady returns the oldest item for each outbox that this instance is ready to send for
func (o *Outboxes) ReadReady(rc redis.Conn) (map[Outbox]*Item, error) {
	pairs, err := redis.ByteSlices(outboxesReadReadyScript.Do(rc, o.allKey(), o.readyKey(), o.KeyBase))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	ready := make(map[Outbox]*Item, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		outbox := string(pairs[i])
		itemJSON := pairs[i+1]

		item := &Item{}
		if err := json.Unmarshal(itemJSON, item); err != nil {
			return nil, fmt.Errorf("error decoding item %s: %v", itemJSON, err)
		}

		ready[decodeOutbox(outbox)] = item
	}

	return ready, nil
}

func (o *Outboxes) RecordSent(rc redis.Conn, ch *models.Channel, chatID models.ChatID, itemID ItemID) (bool, error) {
	outbox := Outbox{ch.UUID, chatID}

	result, err := redis.Strings(outboxesRecordSentScript.Do(rc, o.allKey(), o.outboxKey(outbox), o.readyKey(), outbox.String(), itemID))
	if err != nil {
		return false, err
	}
	if result[0] == "empty" {
		return false, fmt.Errorf("outbox empty for chat %s", chatID)
	}
	if result[0] == "wrong-id" {
		return false, fmt.Errorf("expected item id %s in outbox, found %s", itemID, result[1])
	}
	return result[1] == "true", nil
}

func (o *Outboxes) readyKey() string {
	return fmt.Sprintf("%s:ready:%s", o.KeyBase, o.InstanceID)
}

func (o *Outboxes) allKey() string {
	return fmt.Sprintf("%s:outboxes", o.KeyBase)
}

func (o *Outboxes) outboxKey(box Outbox) string {
	return fmt.Sprintf("%s:outbox:%s", o.KeyBase, box)
}
