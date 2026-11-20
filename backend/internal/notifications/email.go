package notifications

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// EmailSender sends tickets, invoices and itineraries over SMTP.
// The address comes from sr:user:<id>:contact (field "email").
type EmailSender struct {
	rdb  *redis.Client
	addr string // host:port
	from string
	auth smtp.Auth
}

func NewEmailSender(rdb *redis.Client, host string, port int, user, pass, from string) *EmailSender {
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return &EmailSender{rdb: rdb, addr: fmt.Sprintf("%s:%d", host, port), from: from, auth: auth}
}

func (e *EmailSender) Send(ctx context.Context, n Notification) error {
	to, err := e.rdb.HGet(ctx, cache.KeyUserContact(n.UserID), "email").Result()
	if cache.IsNil(err) || to == "" {
		return nil
	}
	if err != nil {
		return err
	}
	if e.addr == ":0" {
		return nil // email disabled
	}
	msg := []byte("From: " + e.from + "\r\nTo: " + to + "\r\nSubject: " + n.Title +
		"\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + n.Body + "\r\n")
	return smtp.SendMail(e.addr, e.auth, e.from, []string{to}, msg)
}
