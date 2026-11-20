-- rate_limit.lua
-- Sliding-window rate limiter on a sorted set (score = request time in ms).
-- KEYS[1] window key
-- ARGV[1] now (ms), ARGV[2] window (ms), ARGV[3] limit, ARGV[4] unique member
-- Returns {allowed(1/0), remaining, retry_after_ms}
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)

if count < limit then
  redis.call('ZADD', key, now, ARGV[4])
  redis.call('PEXPIRE', key, window)
  return {1, limit - count - 1, 0}
end

local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry = 0
if oldest[2] then retry = math.max(0, tonumber(oldest[2]) + window - now) end
return {0, 0, retry}
