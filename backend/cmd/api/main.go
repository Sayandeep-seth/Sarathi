// Command api runs the SARATHI HTTP API, or applies migrations with `api migrate`.

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sarathi/internal/cache"
)

func main() {
	ctx := context.Background()

	redisClient, err := cache.NewRedisClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	defer redisClient.Close()

	log.Println("Sarathi Redis connected successfully")

	// Your existing PostgreSQL initialization goes here.
	// Your existing HTTP server initialization goes here.

	// Keep the application alive as your normal server does.
	_ = os.Signal
	_ = signal.Notify
	_ = syscall.SIGINT
}
