-- KEYS[1]: The Redis key for this user's bucket
-- ARGV[1]: Max capacity of the bucket (how many requests it can hold)
-- ARGV[2]: Leak rate (requests leaked per second)
-- ARGV[3]: Current timestamp (Unix seconds from Go application)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local leak_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

if capacity == nil or leak_rate == nil or now == nil then
    return redis.error_reply("Missing required arguments: capacity, leak_rate, and now.")
end

if capacity <= 0 or leak_rate <= 0 then
    return redis.error_reply("Capacity and leak_rate must be positive numbers.")
end

local bucket = redis.call("HMGET", key, "level", "last_leak_time")
local level = tonumber(bucket[1])
local last_leak_time = tonumber(bucket[2])

if level == nil or last_leak_time == nil then
    level = 0
    last_leak_time = now
    redis.call("EXPIRE", key, 3600)
end

local elapsed = math.max(0, now - last_leak_time)
local leaked_amount = elapsed * leak_rate

level = math.max(0, level - leaked_amount)
last_leak_time = now

local allowed = 0
if (level + 1) <= capacity then
    level = level + 1
    allowed = 1
end

redis.call("HSET", key, "level", level, "last_leak_time", last_leak_time)

return allowed
