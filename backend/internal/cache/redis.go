// Package cache contains a minimal Redis health probe.
//
// Milestone 1 only needs readiness checking, so this speaks just enough RESP
// (AUTH + PING) over the standard library. Milestone 3+ (rate limiting, OTP
// state) will introduce a full client library behind the same package.
package cache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Checker reports whether a Redis instance answers PING.
type Checker struct {
	Addr     string
	Password string
}

// Name implements httpapi.Checker.
func (c Checker) Name() string { return "redis" }

// Check dials Redis, optionally authenticates, and expects +PONG.
func (c Checker) Check(ctx context.Context) error {
	if c.Addr == "" {
		return errors.New("redis address not configured")
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return fmt.Errorf("redis dial: %w", err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	}
	r := bufio.NewReader(conn)

	if c.Password != "" {
		if _, err := fmt.Fprintf(conn, "*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(c.Password), c.Password); err != nil {
			return err
		}
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, "+OK") {
			return errors.New("redis auth rejected")
		}
	}
	if _, err := fmt.Fprint(conn, "*1\r\n$4\r\nPING\r\n"); err != nil {
		return err
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "+PONG") {
		return fmt.Errorf("unexpected redis reply %q", strings.TrimSpace(line))
	}
	return nil
}
