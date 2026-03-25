package auth

import (
	"testing"
	"time"
)

func TestManagerBootstrapAndLoginFlow(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0)
	if manager.IsBootstrapped() {
		t.Fatalf("IsBootstrapped() = true, want false")
	}

	if err := manager.Bootstrap("strong-pass", now); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if !manager.IsBootstrapped() {
		t.Fatalf("IsBootstrapped() = false, want true")
	}

	if err := manager.VerifyPassword("strong-pass"); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	token, expiresAt, err := manager.IssueSession(now)
	if err != nil {
		t.Fatalf("IssueSession() error = %v", err)
	}

	if !expiresAt.After(now) {
		t.Fatalf("expiresAt = %v, want after %v", expiresAt, now)
	}

	if err := manager.ValidateSession(token, now.Add(time.Hour)); err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
}

func TestManagerRejectsInvalidCredentialsAndExpiredSession(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0)
	if err := manager.Bootstrap("strong-pass", now); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if err := manager.VerifyPassword("wrong-pass"); err != ErrInvalidCredentials {
		t.Fatalf("VerifyPassword() error = %v, want %v", err, ErrInvalidCredentials)
	}

	token, _, err := manager.IssueSession(now)
	if err != nil {
		t.Fatalf("IssueSession() error = %v", err)
	}

	if err := manager.ValidateSession(token, now.Add(DefaultSessionTTL+time.Minute)); err != ErrInvalidSession {
		t.Fatalf("ValidateSession() error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestManagerRequiresBootstrap(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := manager.VerifyPassword("strong-pass"); err != ErrBootstrapRequired {
		t.Fatalf("VerifyPassword() error = %v, want %v", err, ErrBootstrapRequired)
	}

	if err := manager.Bootstrap("short", time.Now()); err == nil {
		t.Fatalf("Bootstrap() error = nil, want password validation failure")
	}
}
