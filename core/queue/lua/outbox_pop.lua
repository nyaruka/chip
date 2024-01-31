local queueKey, queuesKey, chatID = KEYS[1], KEYS[2], ARGV[1]

local thisItem = redis.call("LPOP", queueKey)
local nextItem = redis.call("LINDEX", queueKey, 0)

if nextItem == false then
    redis.call("ZREM", queuesKey, chatID)
else
    local ts = tonumber(string.sub(nextItem, 1, string.find(nextItem, "|", 1, true) - 1))
    redis.call("ZADD", queuesKey, ts, chatID)
end

return thisItem