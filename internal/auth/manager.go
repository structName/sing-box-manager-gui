package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName         = "sbm_session"
	ConfigFileName     = "auth.json"
	DefaultSessionTTL  = 7 * 24 * time.Hour
	MinPasswordLength  = 8
	signingKeySize     = 32
)

var (
	ErrAlreadyBootstrapped = errors.New("authentication already bootstrapped")
	ErrBootstrapRequired   = errors.New("authentication bootstrap required")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidSession      = errors.New("invalid session")
)

type Config struct {
	PasswordHash      string `json:"password_hash"`
	SessionSigningKey string `json:"session_signing_key"`
	SessionTTLMinutes int    `json:"session_ttl_minutes"`
	BootstrappedAt    string `json:"bootstrapped_at"`
}

type sessionClaims struct {
	ExpiresAt int64 `json:"exp"`
	IssuedAt  int64 `json:"iat"`
}

type Manager struct {
	configPath string
	mu         sync.RWMutex
	config     Config
}

func NewManager(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create auth directory: %w", err)
	}

	manager := &Manager{
		configPath: filepath.Join(baseDir, ConfigFileName),
	}

	if err := manager.load(); err != nil {
		return nil, err
	}

	if err := manager.ensureSigningKey(); err != nil {
		return nil, err
	}

	return manager, nil
}

func (m *Manager) IsBootstrapped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config.PasswordHash != ""
}

func (m *Manager) SessionTTL() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return time.Duration(m.config.SessionTTLMinutes) * time.Minute
}

func (m *Manager) Bootstrap(password string, now time.Time) error {
	if err := validatePassword(password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.PasswordHash != "" {
		return ErrAlreadyBootstrapped
	}

	m.config.PasswordHash = string(hash)
	m.config.BootstrappedAt = now.UTC().Format(time.RFC3339)
	return m.saveLocked()
}

func (m *Manager) VerifyPassword(password string) error {
	m.mu.RLock()
	hash := m.config.PasswordHash
	m.mu.RUnlock()

	if hash == "" {
		return ErrBootstrapRequired
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	return nil
}

func (m *Manager) IssueSession(now time.Time) (string, time.Time, error) {
	m.mu.RLock()
	ttl := time.Duration(m.config.SessionTTLMinutes) * time.Minute
	signingKey := m.config.SessionSigningKey
	m.mu.RUnlock()

	if signingKey == "" {
		return "", time.Time{}, ErrBootstrapRequired
	}

	expiresAt := now.UTC().Add(ttl)
	claimsBytes, err := json.Marshal(sessionClaims{
		ExpiresAt: expiresAt.Unix(),
		IssuedAt:  now.UTC().Unix(),
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal session: %w", err)
	}

	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signature, err := signToken(payload, signingKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return payload + "." + signature, expiresAt, nil
}

func (m *Manager) ValidateSession(token string, now time.Time) error {
	payload, signature, err := splitToken(token)
	if err != nil {
		return err
	}

	m.mu.RLock()
	signingKey := m.config.SessionSigningKey
	m.mu.RUnlock()

	expected, err := signToken(payload, signingKey)
	if err != nil {
		return err
	}

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrInvalidSession
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ErrInvalidSession
	}

	var claims sessionClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return ErrInvalidSession
	}

	if now.UTC().Unix() >= claims.ExpiresAt {
		return ErrInvalidSession
	}

	return nil
}

func (m *Manager) State(token string, now time.Time) (bool, bool) {
	bootstrapped := m.IsBootstrapped()
	if !bootstrapped || token == "" {
		return bootstrapped, false
	}

	return bootstrapped, m.ValidateSession(token, now) == nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if errors.Is(err, os.ErrNotExist) {
		m.config = Config{
			SessionTTLMinutes: int(DefaultSessionTTL / time.Minute),
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read auth config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse auth config: %w", err)
	}
	if config.SessionTTLMinutes <= 0 {
		config.SessionTTLMinutes = int(DefaultSessionTTL / time.Minute)
	}
	m.config = config
	return nil
}

func (m *Manager) ensureSigningKey() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.SessionSigningKey != "" {
		return nil
	}

	key, err := randomKey()
	if err != nil {
		return err
	}

	m.config.SessionSigningKey = key
	return m.saveLocked()
}

func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0o600); err != nil {
		return fmt.Errorf("write auth config: %w", err)
	}

	return nil
}

func validatePassword(password string) error {
	trimmed := strings.TrimSpace(password)
	if len(trimmed) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}

	return nil
}

func randomKey() (string, error) {
	buf := make([]byte, signingKeySize)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate signing key: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func splitToken(token string) (string, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidSession
	}

	return parts[0], parts[1], nil
}

func signToken(payload, signingKey string) (string, error) {
	key, err := base64.RawURLEncoding.DecodeString(signingKey)
	if err != nil {
		return "", fmt.Errorf("decode signing key: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", fmt.Errorf("sign session: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
