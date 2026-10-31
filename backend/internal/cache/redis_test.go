package cache

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeRedis answers one connection using a scripted reply to each command.
func fakeRedis(t *testing.T, replies ...string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		for _, rep := range replies {
			// consume one RESP array command
			head, _ := r.ReadString('\n')
			n := 0
			for _, ch := range strings.TrimSpace(head)[1:] {
				n = n*10 + int(ch-'0')
			}
			for i := 0; i < n*2; i++ {
				_, _ = r.ReadString('\n')
			}
			_, _ = conn.Write([]byte(rep))
		}
	}()
	return ln.Addr().String()
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestPingOK(t *testing.T) {
	if err := (Checker{Addr: fakeRedis(t, "+PONG\r\n")}).Check(ctx(t)); err != nil {
		t.Fatal(err)
	}
}

func TestAuthThenPing(t *testing.T) {
	if err := (Checker{Addr: fakeRedis(t, "+OK\r\n", "+PONG\r\n"), Password: "pw"}).Check(ctx(t)); err != nil {
		t.Fatal(err)
	}
}

func TestAuthRejected(t *testing.T) {
	if err := (Checker{Addr: fakeRedis(t, "-ERR invalid password\r\n"), Password: "bad"}).Check(ctx(t)); err == nil {
		t.Fatal("expected auth failure")
	}
}

func TestUnreachableAndUnconfigured(t *testing.T) {
	if err := (Checker{Addr: "127.0.0.1:1"}).Check(ctx(t)); err == nil {
		t.Fatal("expected dial failure")
	}
	if err := (Checker{}).Check(ctx(t)); err == nil {
		t.Fatal("expected unconfigured failure")
	}
}
