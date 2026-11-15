// Package auth issues and verifies access tokens, derives boarding codes and
// delivers OTP messages.
package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "sarathi-api"

// Identity is the authenticated caller.
type Identity struct {
	UserID string
	Role   string
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Tokens signs and verifies short-lived HS256 access tokens.
type Tokens struct {
	Secret []byte
	TTL    time.Duration
}

// Issue creates an access token for the user.
func (t Tokens) Issue(userID, role string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuer, Subject: userID,
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(t.TTL)),
		},
	}).SignedString(t.Secret)
}

// Parse validates signature, algorithm, issuer and expiry.
func (t Tokens) Parse(raw string) (Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return t.Secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil {
		return Identity{}, err
	}
	if c.Subject == "" {
		return Identity{}, errors.New("token has no subject")
	}
	return Identity{UserID: c.Subject, Role: c.Role}, nil
}

// NewRefreshToken returns 256 bits of randomness, URL-safe.
func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// BoardingCode derives the 6-digit code a passenger shows the driver. It is an HMAC
// of the trip id, so it needs no storage and cannot be guessed from the id.
func BoardingCode(secret []byte, tripID string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("boarding|" + tripID))
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(m.Sum(nil)[:4])%1_000_000)
}

// CheckBoardingCode compares in constant time.
func CheckBoardingCode(secret []byte, tripID, code string) bool {
	return subtle.ConstantTimeCompare([]byte(BoardingCode(secret, tripID)), []byte(code)) == 1
}

var phoneRE = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// ValidPhone reports whether s is an E.164 number.
func ValidPhone(s string) bool { return phoneRE.MatchString(s) }

// MaskPhone hides all but the last two digits for logs.
func MaskPhone(s string) string {
	if len(s) < 4 {
		return "***"
	}
	return s[:3] + "******" + s[len(s)-2:]
}

// Sender delivers an OTP to a phone.
type Sender interface {
	Send(ctx context.Context, phone, code string) error
}

// LogSender writes the code to the server log. Non-production only (config enforces).
type LogSender struct{ Log *slog.Logger }

// Send implements Sender.
func (s LogSender) Send(_ context.Context, phone, code string) error {
	s.Log.Warn("otp_issued_via_log_sender", slog.String("phone", MaskPhone(phone)), slog.String("code", code))
	return nil
}

// WebhookSender POSTs {"to","message"} to an SMS gateway with a bearer token.
type WebhookSender struct {
	URL, Token string
	Client     *http.Client
}

// Send implements Sender.
func (s WebhookSender) Send(ctx context.Context, phone, code string) error {
	body, _ := json.Marshal(map[string]string{
		"to":      phone,
		"message": "Your SARATHI verification code is " + code + ". It expires in 5 minutes. Do not share it.",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sms gateway returned %d", resp.StatusCode)
	}
	return nil
}
