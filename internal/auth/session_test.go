package auth

import (
	"errors"
	"testing"
	"time"
)

func TestHashPasswordAndCheckPassword(t *testing.T) {
	hashedPassword, err := HashPassword("12345678")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if err := CheckPassword(hashedPassword, "12345678"); err != nil {
		t.Fatalf("CheckPassword() error = %v", err)
	}
}

func TestHashPasswordRejectsShortPassword(t *testing.T) {
	_, err := HashPassword("1234567")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("HashPassword() error = %v, want %v", err, ErrPasswordTooShort)
	}
}

func TestIssueAndValidateSessionToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	token, err := IssueSessionToken("hashed-password", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("IssueSessionToken() error = %v", err)
	}

	claims, err := ValidateSessionToken(token, "hashed-password", now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("ValidateSessionToken() error = %v", err)
	}
	if claims.ExpiresAt != now.Add(10*time.Minute).Unix() {
		t.Fatalf("ValidateSessionToken() expires_at = %d", claims.ExpiresAt)
	}
}

func TestValidateSessionTokenRejectsExpiredToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	token, err := IssueSessionToken("hashed-password", time.Minute, now)
	if err != nil {
		t.Fatalf("IssueSessionToken() error = %v", err)
	}

	_, err = ValidateSessionToken(token, "hashed-password", now.Add(2*time.Minute))
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("ValidateSessionToken() error = %v, want %v", err, ErrSessionExpired)
	}
}

func TestValidateSessionTokenRejectsTamperedToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	token, err := IssueSessionToken("hashed-password", time.Minute, now)
	if err != nil {
		t.Fatalf("IssueSessionToken() error = %v", err)
	}

	_, err = ValidateSessionToken(token+"tampered", "hashed-password", now)
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("ValidateSessionToken() error = %v, want %v", err, ErrSessionInvalid)
	}
}
