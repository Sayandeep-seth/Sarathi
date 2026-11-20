-- reserve_seat.lua
-- Atomic all-or-nothing slot reservation on a bitmap.
-- Used for: daily-bus seats, luxury-bus seats, self-drive & tourist calendar days,
-- premium/vintage hourly slots, test-drive half-hour slots.
--
-- KEYS[1]  bitmap key (bit = 1 => slot permanently booked)
--          derived keys: KEYS[1]..':hold:'..slot (temporary hold, value = owner)
--                        KEYS[1]..':owner'       (HASH slot -> owner of confirmed slot)
--          KEYS[1] carries a {hash-tag}, so derived keys live in the same cluster slot.
-- ARGV[1]  mode: hold | confirm | release | cancel
-- ARGV[2]  owner id (booking / ticket / reservation id)
-- ARGV[3]  hold ttl in seconds (mode=hold)
-- ARGV[4]  capacity (number of slots)
-- ARGV[5..] slot numbers (0-based)
--
-- Returns {code, slot}
--   { 1, -1}  ok
--   { 0,  s}  slot s already booked / held by someone else
--   {-1,  i}  invalid slot number at argument i
--   {-2,  s}  caller does not own the hold / booking of slot s

local bitmap = KEYS[1]
local mode   = ARGV[1]
local owner  = ARGV[2]
local ttl    = tonumber(ARGV[3])
local cap    = tonumber(ARGV[4])
local owners = bitmap .. ':owner'

local function holdkey(s) return bitmap .. ':hold:' .. s end

if #ARGV < 5 then return {-1, 0} end
for i = 5, #ARGV do
  local s = tonumber(ARGV[i])
  if s == nil or s < 0 or s >= cap then return {-1, i} end
end

if mode == 'hold' then
  for i = 5, #ARGV do
    local s = ARGV[i]
    if redis.call('GETBIT', bitmap, s) == 1 then return {0, tonumber(s)} end
    local h = redis.call('GET', holdkey(s))
    if h and h ~= owner then return {0, tonumber(s)} end
  end
  for i = 5, #ARGV do
    redis.call('SET', holdkey(ARGV[i]), owner, 'EX', ttl)
  end
  return {1, -1}

elseif mode == 'confirm' then
  for i = 5, #ARGV do
    local s = ARGV[i]
    if redis.call('GET', holdkey(s)) ~= owner then return {-2, tonumber(s)} end
  end
  for i = 5, #ARGV do
    local s = ARGV[i]
    redis.call('SETBIT', bitmap, s, 1)
    redis.call('HSET', owners, s, owner)
    redis.call('DEL', holdkey(s))
  end
  return {1, -1}

elseif mode == 'release' then
  for i = 5, #ARGV do
    local hk = holdkey(ARGV[i])
    if redis.call('GET', hk) == owner then redis.call('DEL', hk) end
  end
  return {1, -1}

elseif mode == 'cancel' then
  for i = 5, #ARGV do
    if redis.call('HGET', owners, ARGV[i]) ~= owner then return {-2, tonumber(ARGV[i])} end
  end
  for i = 5, #ARGV do
    redis.call('SETBIT', bitmap, ARGV[i], 0)
    redis.call('HDEL', owners, ARGV[i])
  end
  return {1, -1}
end

return redis.error_reply('reserve_seat: unknown mode ' .. tostring(mode))
