package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// signToken creates a simple HMAC-signed token: base64(username):base64(hmac).
func signToken(username, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(username))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	encoded := base64.RawURLEncoding.EncodeToString([]byte(username))
	return encoded + "." + sig
}

// verifyToken validates the token and returns the username if valid.
func verifyToken(token, secret string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	usernameBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	username := string(usernameBytes)

	expected := signToken(username, secret)
	if !hmac.Equal([]byte(token), []byte(expected)) {
		return "", false
	}
	return username, true
}

// signPendingToken creates a short-lived "password verified, awaiting 2FA" token.
// The embedded principal is "2fa:<username>" so a pending token can never be
// mistaken for a full session token (validSession rejects it because no user
// named "2fa:alice" exists).
func signPendingToken(username, secret string) string {
	return signToken("2fa:"+username, secret)
}

// verifyPendingToken validates a pending-2FA token and returns the bare username.
func verifyPendingToken(token, secret string) (string, bool) {
	raw, ok := verifyToken(token, secret)
	if !ok || !strings.HasPrefix(raw, "2fa:") {
		return "", false
	}
	return strings.TrimPrefix(raw, "2fa:"), true
}
