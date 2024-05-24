package queue

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/gocommon/jsonx"
)

//go:embed lua/outbox_pop.lua
var outboxPop string
var outboxPopScript = redis.NewScript(2, outboxPop)

//go:embed lua/outbox_pop_all.lua
var outboxPopAll string
var outboxPopAllScript = redis.NewScript(2, outboxPopAll)

type Outboxes struct {
	KeyBase string
}

func (o *Outboxes) AddMessage(rc redis.Conn, ch *models.Channel, m *models.MsgOut) error {
	rc.Send("MULTI")
	rc.Send("RPUSH", o.chatQueueKey(ch, m.ChatID), o.encodeMsg(m))
	rc.Send("ZADD", o.allChatsKey(), "NX", m.Time.UnixMilli(), fmt.Sprintf("%s:%s", ch.UUID, m.ChatID)) // update only if we're first message in queue
	_, err := rc.Do("EXEC")
	return err
}

type Outbox struct {
	ChannelUUID models.ChannelUUID
	ChatID      models.ChatID
	Oldest      time.Time
}

func (o *Outboxes) All(rc redis.Conn) ([]*Outbox, error) {
	ss, err := redis.Strings(rc.Do("ZRANGE", o.allChatsKey(), "-inf", "+inf", "BYSCORE", "WITHSCORES"))
	if err != nil {
		return nil, err
	}

	boxes := make([]*Outbox, len(ss)/2)
	for i, j := 0, 0; i < len(ss); i += 2 {
		parts := strings.SplitN(ss[i], ":", 2)
		ts, _ := strconv.ParseInt(ss[i+1], 10, 64)

		boxes[j] = &Outbox{
			ChannelUUID: models.ChannelUUID(parts[0]),
			ChatID:      models.ChatID(parts[1]),
			Oldest:      time.UnixMilli(ts).In(time.UTC),
		}

		j++
	}
	return boxes, nil
}

func (o *Outboxes) PopMessage(rc redis.Conn, ch *models.Channel, chatID models.ChatID) (*models.MsgOut, error) {
	item, err := redis.Bytes(outboxPopScript.Do(rc, o.chatQueueKey(ch, chatID), o.allChatsKey(), ch.UUID, chatID))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	return o.decodeMsg(item), nil
}

func (o *Outboxes) PopAll(rc redis.Conn, ch *models.Channel, chatID models.ChatID) ([]*models.MsgOut, error) {
	items, err := redis.ByteSlices(outboxPopAllScript.Do(rc, o.chatQueueKey(ch, chatID), o.allChatsKey(), ch.UUID, chatID))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}

	msgs := make([]*models.MsgOut, len(items))
	for i := range items {
		msgs[i] = o.decodeMsg(items[i])
	}

	return msgs, nil
}

func (o *Outboxes) chatQueueKey(channel *models.Channel, chatID models.ChatID) string {
	return fmt.Sprintf("%s:queue:%s:%s", o.KeyBase, channel.UUID, chatID)
}

func (o *Outboxes) allChatsKey() string {
	return fmt.Sprintf("%s:queues", o.KeyBase)
}

func (o *Outboxes) encodeMsg(msg *models.MsgOut) []byte {
	// queued item payload is <timestamp>|<msg-json>
	var b bytes.Buffer
	b.WriteString(fmt.Sprint(msg.Time.UnixMilli()))
	b.WriteRune('|')
	b.Write(jsonx.MustMarshal(msg))
	return b.Bytes()
}

func (o *Outboxes) decodeMsg(b []byte) *models.MsgOut {
	parts := bytes.SplitN(b, []byte{'|'}, 2)
	m := &models.MsgOut{}
	jsonx.MustUnmarshal(parts[1], m)
	return m
}
