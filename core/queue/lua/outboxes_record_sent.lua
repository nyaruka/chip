local allKey, outboxKey, readyKey, outbox, msgID = KEYS[1], KEYS[2], KEYS[3], ARGV[1], tonumber(ARGV[2])

local thisItem = redis.call("LINDEX", outboxKey, 0)
if thisItem == false then
    return {"empty"}
end

-- check that the id of the message we're removing matches the one we were given
local msg = cjson.decode(thisItem)
if msg["id"] ~= msgID then
    return {"wrong-id", tostring(msg["id"])}
end

-- remove the message from the queue
redis.call("LTRIM", outboxKey, 1, -1)

-- now check if there are any more messages in the queue
local nextItem = redis.call("LINDEX", outboxKey, 0)
local hasMore = false

if nextItem == false then
    -- nothing more in the queue for this chat so take it out of the master set
    redis.call("ZREM", allKey, outbox)
else
    -- update the score of this queue to the timestamp of its new oldest message
    local msg = cjson.decode(nextItem)
    redis.call("ZADD", allKey, msg["_ts"], outbox)
    hasMore = true
end

-- put this chat back in the ready set
redis.call("SADD", readyKey, outbox)

return {"success", tostring(hasMore)}