local queueKey, queuesKey, channelUUID, chatID = KEYS[1], KEYS[2], ARGV[1], ARGV[2]

local items = redis.call("LRANGE", queueKey, 0, -1)

redis.call("DEL", queueKey)
redis.call("ZREM", queuesKey, channelUUID .. ":" .. chatID)

return items