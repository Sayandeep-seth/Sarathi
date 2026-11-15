package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestTokenRoundTrip(t *testing.T) {
	tk := Tokens{Secret: secret, TTL: time.Minute}
	raw, err := tk.Issue("user-1", "driver")
	if err != nil {
		t.Fatal(err)
	}
	id, err := tk.Parse(raw)
	if err != nil || id.UserID != "user-1" || id.Role != "driver" {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestTokenRejections(t *testing.T) {
	tk := Tokens{Secret: secret, TTL: time.Minute}
	good, _ := tk.Issue("u", "passenger")

	expired, _ := Tokens{Secret: secret, TTL: -time.Minute}.Issue("u", "passenger")
	otherKey, _ := Tokens{Secret: []byte("another-secret-another-secret-123"), TTL: time.Minute}.Issue("u", "passenger")
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "u", "iss": issuer, "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	parts := strings.Split(good, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]

	for name, raw := range map[string]string{
		"expired": expired, "wrong key": otherKey, "alg none": none, "tampered": tampered, "garbage": "abc",
	} {
		if _, err := tk.Parse(raw); err == nil {
			t.Errorf("%s token was accepted", name)
		}
	}
}

func TestBoardingCode(t *testing.T) {
	a := BoardingCode(secret, "trip-a")
	if len(a) != 6 || a != BoardingCode(secret, "trip-a") {
		t.Fatalf("code not stable 6 digits: %q", a)
	}
	if !CheckBoardingCode(secret, "trip-a", a) || CheckBoardingCode(secret, "trip-b", a) && a != BoardingCode(secret, "trip-b") {
		t.Fatal("boarding code verification is wrong")
	}
	if CheckBoardingCode(secret, "trip-a", "") {
		t.Fatal("empty code accepted")
	}
}

func TestPhoneValidationAndMask(t *testing.T) {
	for _, ok := range []string{"+919876543210", "+14155550123"} {
		if !ValidPhone(ok) {
			t.Errorf("%s rejected", ok)
		}
	}
	for _, bad := range []string{"9876543210", "+0123456789", "+91 98765", "+91abc", ""} {
		if ValidPhone(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if m := MaskPhone("+919876543210"); strings.Contains(m, "98765") {
		t.Fatalf("mask leaks digits: %s", m)
	}
}
