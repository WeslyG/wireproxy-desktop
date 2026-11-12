package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == applyUpdateFlag {
		if err := runUpdateHelper(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(args) >= 2 && args[0] == "--cleanup-update" {
		exe, err := os.Executable()
		if err != nil {
			os.Exit(1)
		}
		cleanUpdateHelper(exe, args[1])
		result := fmt.Sprintf("portable=%t failed=%t", slices.Contains(args, "--portable"), slices.Contains(args, "--update-error"))
		if err := os.WriteFile(filepath.Join(filepath.Dir(exe), "restarted.txt"), []byte(result), 0600); err != nil {
			os.Exit(1)
		}
		return
	}
	os.Exit(m.Run())
}

func TestUpdateDiscoveryDownloadInstallAndRestart(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	old := payload

	for _, tc := range []struct {
		name         string
		payload      []byte
		wantRollback bool
	}{
		{"valid release", payload, false},
		{"invalid executable rolls back", []byte("not an executable"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, platformAsset(runtime.GOOS, runtime.GOARCH))
			if err := os.WriteFile(target, old, 0700); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(tc.payload))
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/latest":
					_ = json.NewEncoder(w).Encode(desktopRelease{Tag: "v1.11.0", Assets: []releaseAsset{
						{Name: "other-platform", URL: "https://" + r.Host + "/wrong"},
						{Name: filepath.Base(target), URL: "https://" + r.Host + "/binary", Size: int64(len(tc.payload)), Digest: digest},
					}})
				case "/binary":
					_, _ = w.Write(tc.payload)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			for _, current := range []string{"v1.11.0", "v1.12.0", "dev"} {
				release, err := fetchRelease(ctx, server.Client(), server.URL+"/latest", current, filepath.Base(target))
				if err != nil || release != nil {
					t.Fatalf("offered an update for %s: %v, %v", current, release, err)
				}
			}
			release, err := fetchRelease(ctx, server.Client(), server.URL+"/latest", "v1.9.0", filepath.Base(target))
			if err != nil || release == nil || release.Tag != "v1.11.0" {
				t.Fatalf("did not discover the new release: %v, %v", release, err)
			}
			wrong := release.Asset
			wrong.Digest = "sha256:" + strings.Repeat("0", 64)
			if _, err := stageUpdate(ctx, server.Client(), target, wrong); err == nil {
				t.Fatal("accepted a binary with a mismatched SHA-256")
			}
			staged, err := stageUpdate(ctx, server.Client(), target, release.Asset)
			if err != nil {
				t.Fatal(err)
			}
			stagedInfo, err := os.Stat(staged)
			if err != nil {
				t.Fatal(err)
			}
			handoff, err := startUpdateHelper(target, staged, release.Asset.Digest, true)
			if err != nil {
				t.Fatal(err)
			}
			defer handoff.Close()
			if info, err := os.Stat(target); err != nil || os.SameFile(info, stagedInfo) {
				t.Fatal("the running executable was replaced before shutdown")
			}
			if err := handoff.Close(); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(dir, "restarted.txt")
			for {
				if info, err := os.Stat(marker); err == nil && info.Size() > 0 {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("the app did not restart after installation")
				case <-time.After(25 * time.Millisecond):
				}
			}
			result, err := os.ReadFile(marker)
			if err != nil || string(result) != fmt.Sprintf("portable=true failed=%t", tc.wantRollback) {
				t.Fatalf("unexpected restart result: %q (%v)", result, err)
			}
			want := tc.payload
			if tc.wantRollback {
				want = old
			}
			if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, want) {
				t.Fatal("the installed executable has unexpected contents")
			}
			if info, err := os.Stat(target); err != nil || os.SameFile(info, stagedInfo) == tc.wantRollback {
				t.Fatal("the executable was not replaced or rolled back as expected")
			}
		})
	}
}
