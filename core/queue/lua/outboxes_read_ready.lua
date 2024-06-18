local allKey, readyKey, keyBase = KEYS[1], KEYS[2], ARGV[1]

local outboxes = redis.call("ZINTER", 2, allKey, readyKey)

local result = {} -- pairs of queue IDs and items

for i, outbox in ipairs(outboxes) do
    local item = redis.call("LINDEX", keyBase .. ":outbox:" .. outbox, 0)

    table.insert(result, outbox)
    table.insert(result, item)

    redis.call("SREM", readyKey, outbox)
end

return result