package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	valkey "github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/dates"
)

const (
	// how long a conversation has to go quiet before its unread messages are emailed, so a burst goes as one email.
	// A visitor with the chat open may be reading along, so gets longer. One without it open has already been gone a
	// while by the time their chat's presence lapses, and for them email is the conversation - just long enough for a
	// chat that's only starting to load what it's sent from history.
	quietPeriodConnected = 3 * time.Minute
	quietPeriodOffline   = 10 * time.Second

	// the longest a message is held back however chatty the conversation stays
	maxHold = 30 * time.Minute

	// how long a conversation's pending state outlives its last change, in case it's never claimed
	pendingTTL = 24 * time.Hour
)

// a conversation's pending email is a hash of its address, when its oldest message was held back and how many sends
// have failed, plus the set of messages held back. Conversations are queued for sending in one sorted set, scored by
// when they're due. While a failed send waits to be retried, its due time only moves later, so holding back another
// message never cuts that wait short.
const keyDue = "chat-email-due"

func pendingKeys(ch models.ChannelUUID, chatID string) (member, state, msgs string) {
	member = fmt.Sprintf("%s:%s", ch, chatID)
	return member, "chat-email:" + member, "chat-email-msgs:" + member
}

// KEYS: due, state, msgs. ARGV: member, msg UUID, address, now, quiet period, max hold, TTL
var holdScript = valkey.NewScript(3, `
redis.call("SADD", KEYS[3], ARGV[2])
redis.call("HSET", KEYS[2], "email", ARGV[3])
redis.call("HSETNX", KEYS[2], "first", ARGV[4])
local first = tonumber(redis.call("HGET", KEYS[2], "first"))
local due = math.min(tonumber(ARGV[4]) + tonumber(ARGV[5]), first + tonumber(ARGV[6]))
if redis.call("HEXISTS", KEYS[2], "attempts") == 1 then
	redis.call("ZADD", KEYS[1], "GT", due, ARGV[1])
else
	redis.call("ZADD", KEYS[1], due, ARGV[1])
end
redis.call("EXPIRE", KEYS[2], ARGV[7])
redis.call("EXPIRE", KEYS[3], ARGV[7])
`)

// KEYS: due, state, msgs. ARGV: member
var dropScript = valkey.NewScript(3, `
redis.call("ZREM", KEYS[1], ARGV[1])
redis.call("DEL", KEYS[2], KEYS[3])
`)

// KEYS: due, state, msgs. ARGV: member, now. Only one claimer can see a due conversation's state, as claiming removes it.
var claimScript = valkey.NewScript(3, `
local due = redis.call("ZSCORE", KEYS[1], ARGV[1])
if not due or tonumber(due) > tonumber(ARGV[2]) then
	return false
end
redis.call("ZREM", KEYS[1], ARGV[1])
local state = redis.call("HGETALL", KEYS[2])
local msgs = redis.call("SMEMBERS", KEYS[3])
redis.call("DEL", KEYS[2], KEYS[3])
return {state, msgs}
`)

// KEYS: due, state, msgs. ARGV: member, address, attempts, now, due, TTL, msg UUIDs... Anything held back while the
// failed send was in flight is merged in, keeping its newer address.
var requeueScript = valkey.NewScript(3, `
redis.call("SADD", KEYS[3], unpack(ARGV, 7))
redis.call("HSETNX", KEYS[2], "email", ARGV[2])
redis.call("HSET", KEYS[2], "attempts", ARGV[3])
redis.call("HSETNX", KEYS[2], "first", ARGV[4])
redis.call("ZADD", KEYS[1], "GT", ARGV[5], ARGV[1])
redis.call("EXPIRE", KEYS[2], ARGV[6])
redis.call("EXPIRE", KEYS[3], ARGV[6])
`)

// Hold holds back a message delivered to a conversation, to be emailed to the given address if the visitor hasn't
// read it once the conversation goes quiet - for which how long depends on whether the visitor has the chat open
func Hold(ctx context.Context, rt *runtime.Runtime, ch models.ChannelUUID, chatID string, msg models.MsgUUID, address string, connected bool) error {
	vc, err := rt.VK.GetContext(ctx)
	if err != nil {
		return err
	}
	defer vc.Close()

	quietPeriod := quietPeriodOffline
	if connected {
		quietPeriod = quietPeriodConnected
	}

	member, state, msgs := pendingKeys(ch, chatID)
	_, err = holdScript.Do(vc, keyDue, state, msgs,
		member, string(msg), address, dates.Now().Unix(), int(quietPeriod.Seconds()), int(maxHold.Seconds()), int(pendingTTL.Seconds()))
	return err
}

// Drop drops any messages held back for a conversation, e.g. because the contact no longer has an address
func Drop(ctx context.Context, rt *runtime.Runtime, ch models.ChannelUUID, chatID string) error {
	vc, err := rt.VK.GetContext(ctx)
	if err != nil {
		return err
	}
	defer vc.Close()

	member, state, msgs := pendingKeys(ch, chatID)
	_, err = dropScript.Do(vc, keyDue, state, msgs, member)
	return err
}

// batch is a conversation's held back messages, claimed for sending
type batch struct {
	ChannelUUID models.ChannelUUID
	ChatID      string
	Address     string
	Attempts    int
	MsgUUIDs    []models.MsgUUID
}

// dueMembers returns up to the given number of conversations which are due to be emailed
func dueMembers(vc valkey.Conn, limit int) ([]string, error) {
	return valkey.Strings(vc.Do("ZRANGEBYSCORE", keyDue, "-inf", dates.Now().Unix(), "LIMIT", 0, limit))
}

// claim claims the given conversation's held back messages, returning nil if it isn't due or another claimer won it
func claim(vc valkey.Conn, member string) (*batch, error) {
	ch, chatID, _ := strings.Cut(member, ":")
	_, state, msgs := pendingKeys(models.ChannelUUID(ch), chatID)

	reply, err := valkey.Values(claimScript.Do(vc, keyDue, state, msgs, member, dates.Now().Unix()))
	if err == valkey.ErrNil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	fields, err := valkey.StringMap(reply[0], nil)
	if err != nil {
		return nil, err
	}
	uuids, err := valkey.Strings(reply[1], nil)
	if err != nil {
		return nil, err
	}

	b := &batch{ChannelUUID: models.ChannelUUID(ch), ChatID: chatID, Address: fields["email"]}
	fmt.Sscan(fields["attempts"], &b.Attempts)
	for _, u := range uuids {
		b.MsgUUIDs = append(b.MsgUUIDs, models.MsgUUID(u))
	}
	return b, nil
}

// requeue puts a batch whose send failed back to be tried again after the given delay
func requeue(vc valkey.Conn, b *batch, delay time.Duration) error {
	if len(b.MsgUUIDs) == 0 {
		return nil
	}

	member, state, msgs := pendingKeys(b.ChannelUUID, b.ChatID)
	now := dates.Now()

	args := []any{keyDue, state, msgs, member, b.Address, b.Attempts, now.Unix(), now.Add(delay).Unix(), int(pendingTTL.Seconds())}
	for _, u := range b.MsgUUIDs {
		args = append(args, string(u))
	}
	_, err := requeueScript.Do(vc, args...)
	return err
}
