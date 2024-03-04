local queuesKey = KEYS[1]
local keyBase, beforeTS = ARGV[1], ARGV[2]

-- query the oldest queue, older than the given timestamp
local oldest = redis.call("ZRANGE", queuesKey, "-inf", beforeTS, "BYSCORE", "LIMIT", 0, 1)

if oldest ~= false then
    local queue = oldest[1]
    local queueKey = keyBase .. ":queue:" .. queue
    local items = redis.call("LRANGE", queueKey, 0, -1)

    redis.call("ZREM", queuesKey, queueKey)
    redis.call("DEL", queueKey)

    table.insert(items, 1, queue) -- insert queue id at start of array

    return items
end

return false