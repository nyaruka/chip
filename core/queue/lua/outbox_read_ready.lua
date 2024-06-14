local queuesKey, readyKey, queuePrefix = KEYS[1], KEYS[2], ARGV[1]

local chatIDs = redis.call("ZINTER", 2, queuesKey, readyKey)

local msgs = {}

for i, chatID in ipairs(chatIDs) do
    local chatMsg = redis.call("LINDEX", queuePrefix .. chatID, 0)

    msgs[i] = chatMsg

    redis.call("SREM", readyKey, chatID)
end

return msgs