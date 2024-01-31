local queueKey, queuesKey, chatID = KEYS[1], KEYS[2], ARGV[1]

local items = redis.call("LRANGE", queueKey, 0, -1)

redis.call("DEL", queueKey)
redis.call("ZREM", queuesKey, chatID)

return items