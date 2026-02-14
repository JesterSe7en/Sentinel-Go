-- KEYS[1]: The Redis key for this user's bucket
-- ARGV[1]: Max capacity of the bucket
-- ARGV[2]: Refill rate (tokens per second)
-- ARGV[3]: Current timestamp (from Go application)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- input validation
if capacity == nil or refill_rate == nil or now == nil then
    return redis.error_reply("Missing required arguments")
end

if capacity <= 0 or refill_rate <= 0 then
    return redis.error_reply("Capacity and refill rate must be positive")
end

--------------------------------------------------------------------

local bucket = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    redis.call("EXPIRE", key, 3600)
else
    local elapsed = math.max(0, now - last_refill)
    local tokens_to_add = elapsed * refill_rate
    tokens = math.min(capacity, tokens + tokens_to_add)
end

last_refill = now

local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

redis.call("HSET", key, "tokens", tokens, "last_refill", last_refill)

return allowed
