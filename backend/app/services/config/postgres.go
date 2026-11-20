package config

// PostgresConfig is the primary (source-of-truth) database. Redis sits in front of it.
type PostgresConfig struct {
	DSN           string
	MaxConns      int32
	MigrationsDir string
}

func loadPostgres() PostgresConfig {
	return PostgresConfig{
		DSN:           getEnv("POSTGRES_DSN", "postgres://sarathi:sarathi@localhost:5432/sarathi?sslmode=disable"),
		MaxConns:      int32(getInt("POSTGRES_MAX_CONNS", 20)),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "migrations"),
	}
}
