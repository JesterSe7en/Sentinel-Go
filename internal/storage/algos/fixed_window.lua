local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call("INCR", key)
if current == 1 then
    redis.call("EXPIRE", key, window)
end



-- gRPC response expects allowed, limit, remaining, reset

return { current <= limit, limit, limit - current, window }
