-- KEYS[1]: Base key for rate limiting (e.g., "rate_limit:user:12345")
-- ARGV[1]: Maximum number of requests allowed in the window (limit)
-- ARGV[2]: Size of the sliding window in seconds (windowSize)
-- ARGV[3]: Size of each time bucket in seconds (bucketSize)
-- ARGV[4]: Current timestamp in seconds (now)

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local windowSize = tonumber(ARGV[2])
local bucketSize = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

if limit == nil or windowSize == nil or bucketSize == nil then
    return redis.error_reply("Missing required arguments")
end

if limit <= 0 or windowSize <= 0 or bucketSize <= 0 then
    return redis.error_reply("Limit, window size, and bucket size must be positive")
end

local current_bucket_id = math.floor(now / bucketSize)
local previous_bucket_id = current_bucket_id - 1

local current_key = key .. ":" .. current_bucket_id
local previous_key = key .. ":" .. previous_bucket_id

local current_count = tonumber(redis.call("GET", current_key)) or 0
local previous_count = tonumber(redis.call("GET", previous_key)) or 0

local elapsed_in_current_bucket = now % bucketSize
local time_in_previous_bucket = bucketSize - elapsed_in_current_bucket
local overlap_percentage = time_in_previous_bucket / bucketSize

local estimated_count = current_count + (previous_count * overlap_percentage)


if estimated_count < limit then
    redis.call("INCR", current_key)
    redis.call("EXPIRE", current_key, bucketSize * 2)
end

-- gRPC response expects allowed, limit, remaining, reset
return { estimated_count <= limit, limit, limit - estimated_count, windowSize }
