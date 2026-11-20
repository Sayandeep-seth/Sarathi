-- release_lock.lua
-- Delete a lock only if it is still owned by the caller's token.
-- Prevents a slow client from releasing a lock that already expired and was
-- re-acquired by someone else.
-- KEYS[1] lock key, ARGV[1] owner token. Returns 1 if released, 0 otherwise.
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
