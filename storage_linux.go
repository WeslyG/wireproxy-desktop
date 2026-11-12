package main

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const secretServiceLabel = "Wireproxy Desktop"

func readProtectedConfig(scope string) ([]byte, error) {
	value, err := keyring.Get(secretServiceLabel, scope)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, errConfigMissing
	}
	if err != nil {
		return nil, fmt.Errorf("Secret Service: %w", err)
	}
	return []byte(value), nil
}

func writeProtectedConfig(scope string, data []byte) error {
	if err := keyring.Set(secretServiceLabel, scope, string(data)); err != nil {
		return fmt.Errorf("Secret Service: %w", err)
	}
	return nil
}

func protectedStoreName() string { return "Linux Secret Service" }
