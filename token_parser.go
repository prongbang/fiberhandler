package fiberhandler

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
	"github.com/prongbang/gopkg/core"
)

type TokenParser[T any] interface {
	ParseToken(tokenString string) (*T, error)
}

type JWTParser[T any] struct{}

func (f *JWTParser[T]) ParseToken(tokenString string) (*T, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if needed
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims core.Model[T]
	if err := json.Unmarshal(decoded, &claims.Type); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}

	return &claims.Type, nil
}

func NewJWTParser[T any]() TokenParser[T] {
	return &JWTParser[T]{}
}
