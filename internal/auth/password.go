package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const MinPasswordLength = 8

var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func CheckPassword(hash string, password string) error {
	if hash == "" {
		return errors.New("password hash is empty")
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
