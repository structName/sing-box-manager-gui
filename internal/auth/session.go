package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const CookieName = "sbm_session"

var (
	ErrSessionInvalid = errors.New("session token is invalid")
	ErrSessionExpired = errors.New("session token is expired")
)

type SessionClaims struct {
	Version   int   `json:"version"`
	IssuedAt  int64 `json:"issued_at"`
	ExpiresAt int64 `json:"expires_at"`
}

func IssueSessionToken(passwordHash string, ttl time.Duration, now time.Time) (string, error) {
	if passwordHash == "" {
		return "", errors.New("password hash is empty")
	}
	if ttl <= 0 {
		return "", errors.New("session ttl must be positive")
	}

	claims := SessionClaims{
		Version:   1,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
	return encodeSignedClaims(passwordHash, claims)
}

func ValidateSessionToken(token string, passwordHash string, now time.Time) (SessionClaims, error) {
	if passwordHash == "" {
		return SessionClaims{}, ErrSessionInvalid
	}

	claims, err := decodeSignedClaims(token, passwordHash)
	if err != nil {
		return SessionClaims{}, err
	}
	if claims.ExpiresAt < now.Unix() {
		return SessionClaims{}, ErrSessionExpired
	}

	return claims, nil
}

func encodeSignedClaims(passwordHash string, claims SessionClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := signPayload(passwordHash, encodedPayload)
	return fmt.Sprintf("%s.%s", encodedPayload, signature), nil
}

func decodeSignedClaims(token string, passwordHash string) (SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return SessionClaims{}, ErrSessionInvalid
	}

	expectedSignature := signPayload(passwordHash, parts[0])
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[1])) {
		return SessionClaims{}, ErrSessionInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return SessionClaims{}, ErrSessionInvalid
	}

	var claims SessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return SessionClaims{}, ErrSessionInvalid
	}
	if claims.Version != 1 {
		return SessionClaims{}, ErrSessionInvalid
	}

	return claims, nil
}

func signPayload(passwordHash string, encodedPayload string) string {
	mac := hmac.New(sha256.New, deriveSigningKey(passwordHash))
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func deriveSigningKey(passwordHash string) []byte {
	sum := sha256.Sum256([]byte(passwordHash))
	return sum[:]
}
