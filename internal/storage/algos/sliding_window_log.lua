-- KEYS[1]: Key for rate limiting (e.g., "rate_limit:user:12345")
-- ARGV[1]: Maximum number of requests allowed in the window (limit)
-- ARGV[2]: Size of the sliding window in seconds (windowSize)
-- ARGV[3]: Current timestamp in seconds (now)

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local windowSize = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

if limit == nil or windowSize == nil then
    return redis.error_reply("Missing required arguments")
end

if limit <= 0 or windowSize <= 0 then
    return redis.error_reply("Limit and window size must be positive")
end

local window_start = now - windowSize

redis.call("ZREMRANGEBYSCORE", key, 0, window_start)

local current_count = redis.call("ZCARD", key)

if current_count < limit then
	redis.call("ZADD", key, now, now)
	redis.call("EXPIRE", key, windowSize)
	return 1
end
return 0
