package vesta

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// VerifyWebhookSignature checks X-Vesta-Signature (hex HMAC-SHA256 of raw body).
func VerifyWebhookSignature(secret string, body []byte, signatureHex string) bool {
	if secret == "" || signatureHex == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signatureHex)) == 1
}
