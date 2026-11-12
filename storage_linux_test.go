package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestLinuxProtectedStorage(t *testing.T) {
	if os.Getenv("WIREPROXY_TEST_SECRET_SERVICE") != "1" {
		t.Skip("requires an unlocked desktop Secret Service")
	}
	scope := fmt.Sprintf("test-%d", time.Now().UnixNano())
	if _, err := readProtectedConfig(scope); !errors.Is(err, errConfigMissing) {
		t.Fatalf("unexpected initial value: %v", err)
	}
	t.Cleanup(func() { _ = keyring.Delete(secretServiceLabel, scope) })
	for _, data := range [][]byte{[]byte(`{"test":"first"}`), []byte(`{"test":"updated"}`)} {
		if err := writeProtectedConfig(scope, data); err != nil {
			t.Fatal(err)
		}
		got, err := readProtectedConfig(scope)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("Secret Service round trip failed: %v", err)
		}
	}
}
