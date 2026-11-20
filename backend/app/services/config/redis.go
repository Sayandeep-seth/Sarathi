package config

import "time"

// RedisConfig configures the single Redis deployment used for caching, geo, locks,
// pub/sub, streams and rate limiting.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func loadRedis() RedisConfig {
	return RedisConfig{
		Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           getInt("REDIS_DB", 0),
		PoolSize:     getInt("REDIS_POOL_SIZE", 50),
		MinIdleConns: getInt("REDIS_MIN_IDLE", 10),
		DialTimeout:  getDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:  getDuration("REDIS_READ_TIMEOUT", 3*time.Second),
		WriteTimeout: getDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
	}
}
