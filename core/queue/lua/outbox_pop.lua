local queueKey, queuesKey, chatID = KEYS[1], KEYS[2], ARGV[1]

local msg = redis.call("LPOP", queueKey)
local rem = redis.call("LLEN", queueKey)

if (rem == 0) then
    redis.call("ZREM", queuesKey, chatID)
end

return msg