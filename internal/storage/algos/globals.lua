--- @meta
--- This file provides type hints for the Redis Lua environment

--- @class redis
redis = {}
--- @param command string
--- @return any
function redis.call(command, ...) end
function redis.pcall(command, ...) end
function redis.error_reply(msg) end

--- @type table
KEYS = {}
--- @type table
ARGV = {}

-- Code that calls the script
-- 	result, err := se.script.Run(ctx, se.client, []string{key}, limit, window).Int()
