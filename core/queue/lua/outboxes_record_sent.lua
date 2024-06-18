local allKey, outboxKey, readyKey, outbox, itemID = KEYS[1], KEYS[2], KEYS[3], ARGV[1], ARGV[2]

local thisItem = redis.call("LINDEX", outboxKey, 0)
if thisItem == false then
    return {"empty"}
end

-- check that the id of the item we're removing matches the one we were given
local item = cjson.decode(thisItem)
if item["id"] ~= itemID then
    return {"wrong-id", item["id"]}
end

-- remove the item from the outbox
redis.call("LTRIM", outboxKey, 1, -1)

-- now check if there are any more items in the outbox
local nextItem = redis.call("LINDEX", outboxKey, 0)
local hasMore = false

if nextItem == false then
    -- nothing more in the outbox for this chat so take it out of the master set
    redis.call("ZREM", allKey, outbox)
else
    -- update the score of this outbox to the timestamp of its new oldest item
    local item = cjson.decode(nextItem)
    redis.call("ZADD", allKey, item["ts"], outbox)
    hasMore = true
end

-- put this outbox back in the ready set
redis.call("SADD", readyKey, outbox)

return {"success", tostring(hasMore)}