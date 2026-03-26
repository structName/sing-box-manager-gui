package storage

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const zashboardSecretBytes = 24

func NewZashboardSecret() (string, error) {
	buffer := make([]byte, zashboardSecretBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate zashboard secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
