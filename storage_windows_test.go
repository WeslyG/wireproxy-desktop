package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestWindowsProtectedStorage(t *testing.T) {
	scope := fmt.Sprintf("test-%d", time.Now().UnixNano())
	if _, err := readProtectedConfig(scope); !errors.Is(err, errConfigMissing) {
		t.Fatalf("unexpected initial value: %v", err)
	}
	t.Cleanup(func() {
		if key, err := registry.OpenKey(registry.CURRENT_USER, configRegistryPath, registry.SET_VALUE); err == nil {
			_ = key.DeleteValue(scope)
			_ = key.Close()
		}
	})
	data := []byte(`{"password":"synthetic-storage-test"}`)
	if err := writeProtectedConfig(scope, data); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, configRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	ciphertext, _, err := key.GetBinaryValue(scope)
	if err != nil || bytes.Contains(ciphertext, []byte("synthetic-storage-test")) {
		t.Fatal("registry value is missing or exposes the test secret")
	}
	got, err := readProtectedConfig(scope)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("DPAPI round trip failed: %v", err)
	}
}
