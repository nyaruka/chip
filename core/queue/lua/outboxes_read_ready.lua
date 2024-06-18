local queuesKey, readyKey, keyBase = KEYS[1], KEYS[2], ARGV[1]

local queueIDs = redis.call("ZINTER", 2, queuesKey, readyKey)

local result = {} -- pairs of queue IDs and items

for i, queueID in ipairs(queueIDs) do
    local item = redis.call("LINDEX", keyBase .. ":queue:" .. queueID, 0)

    table.insert(result, queueID)
    table.insert(result, item)

    redis.call("SREM", readyKey, queueID)
end

return result