package transit_pass

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"sarathi/internal/common/utils"
)

// QRGenerator issues signed, rotating QR tokens.
//
// token = base64url("<journeyID>|<nonce>|<expiresUnix>") + "." + hex(HMAC-SHA256)[:32]
//
// The signature lets scanners reject forged codes offline; the nonce is checked
// in Redis so only the newest QR of a journey is valid (no screenshot sharing).
type QRGenerator struct {
	key   []byte
	store *Store
}

func NewQRGenerator(signingKey string, s *Store) *QRGenerator {
	return &QRGenerator{key: []byte(signingKey), store: s}
}

// QR is what the app shows.
type QR struct {
	Token     string    `json:"token"`
	PNG       []byte    `json:"png"` // base64 in JSON
	ExpiresAt time.Time `json:"expires_at"`
}

// Issue creates a fresh QR for an active journey.
func (g *QRGenerator) Issue(ctx context.Context, j *Journey) (*QR, error) {
	nonce := utils.RandomToken(8)
	exp := j.ValidUntil
	payload := fmt.Sprintf("%s|%s|%d", j.ID, nonce, exp.Unix())
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + g.sign(payload)
	if err := g.store.RotateQR(ctx, j.ID, nonce, time.Until(exp)); err != nil {
		return nil, err
	}
	png, err := qrcode.Encode(token, qrcode.Medium, 320)
	if err != nil {
		return nil, err
	}
	return &QR{Token: token, PNG: png, ExpiresAt: exp}, nil
}

func (g *QRGenerator) sign(payload string) string {
	m := hmac.New(sha256.New, g.key)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// parsed token
type tokenData struct {
	JourneyID string
	Nonce     string
	Expires   time.Time
}

// Parse verifies the signature and expiry of a token.
func (g *QRGenerator) Parse(token string) (*tokenData, bool) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || !hmac.Equal([]byte(g.sign(string(raw))), []byte(sig)) {
		return nil, false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return nil, false
	}
	var exp int64
	if _, err := fmt.Sscan(parts[2], &exp); err != nil || time.Now().Unix() > exp {
		return nil, false
	}
	return &tokenData{JourneyID: parts[0], Nonce: parts[1], Expires: time.Unix(exp, 0)}, true
}
