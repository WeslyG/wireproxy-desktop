package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

var version = "dev"
var releaseAPI = "https://api.github.com/repos/WeslyG/wireproxy-desktop/releases/latest"

const (
	applyUpdateFlag    = "--apply-update"
	updateHelperPrefix = ".wireproxy-updater-"
	maxUpdateSize      = 100 << 20
)

var releaseTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type desktopRelease struct {
	Tag    string         `json:"tag_name"`
	Notes  string         `json:"body"`
	Assets []releaseAsset `json:"assets"`
	Asset  releaseAsset   `json:"-"`
}

func stableVersion(tag string) ([3]uint64, bool) {
	var version [3]uint64
	parts := releaseTag.FindStringSubmatch(tag)
	if parts == nil {
		return version, false
	}
	for i := range version {
		n, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return version, false
		}
		version[i] = n
	}
	return version, true
}

func isNewer(tag, current string) bool {
	candidate, ok := stableVersion(tag)
	if !ok {
		return false
	}
	installed, ok := stableVersion(current)
	if !ok {
		return false
	}
	for i := range candidate {
		if candidate[i] != installed[i] {
			return candidate[i] > installed[i]
		}
	}
	return false
}

func platformAsset(goos, arch string) string {
	switch goos {
	case "windows":
		return "wireproxy-desktop-windows-" + arch + ".exe"
	case "darwin":
		return "wireproxy-desktop-macos-" + arch
	default:
		return "wireproxy-desktop-" + goos + "-" + arch
	}
}

func fetchRelease(ctx context.Context, client *http.Client, endpoint, current, assetName string) (*desktopRelease, error) {
	resp, err := updateGET(ctx, client, endpoint, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release check returned HTTP %d", resp.StatusCode)
	}
	var release desktopRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("read release metadata: %w", err)
	}
	if _, ok := stableVersion(release.Tag); !ok {
		return nil, nil
	}
	if !isNewer(release.Tag, current) {
		return &release, nil
	}
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			release.Asset = asset
			return &release, nil
		}
	}
	return &release, errors.New("the release has no binary for this platform")
}

func updateGET(ctx context.Context, client *http.Client, endpoint, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "Wireproxy-Desktop/"+version)
	return client.Do(req)
}

func validDigest(digest string) bool {
	value, ok := strings.CutPrefix(digest, "sha256:")
	raw, err := hex.DecodeString(value)
	return ok && err == nil && len(raw) == sha256.Size
}

func stageUpdate(ctx context.Context, client *http.Client, target string, asset releaseAsset) (string, error) {
	if !validDigest(asset.Digest) || asset.Size < 1 || asset.Size > maxUpdateSize {
		return "", errors.New("the release binary has missing or invalid verification metadata")
	}
	if !strings.HasPrefix(asset.URL, "https://") {
		return "", errors.New("the release binary URL must use HTTPS")
	}
	resp, err := updateGET(ctx, client, asset.URL, "application/octet-stream")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".wireproxy-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot write beside the executable: %w", err)
	}
	defer staged.Close()
	keep := false
	defer func() {
		if !keep {
			_ = staged.Close()
			_ = os.Remove(staged.Name())
		}
	}()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(staged, hash), io.LimitReader(resp.Body, asset.Size+1))
	if err != nil {
		return "", err
	}
	if size != asset.Size || !strings.EqualFold("sha256:"+hex.EncodeToString(hash.Sum(nil)), asset.Digest) {
		return "", errors.New("download failed SHA-256 or size verification")
	}
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		return "", err
	}
	if err := staged.Sync(); err != nil {
		return "", err
	}
	if err := staged.Close(); err != nil {
		return "", err
	}
	keep = true
	return staged.Name(), nil
}

var updateClient = &http.Client{
	Timeout: 2 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) >= 10 {
			return errors.New("invalid update redirect")
		}
		return nil
	},
}

func copyExecutable(source, pattern string) (string, error) {
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(source), pattern)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(file, input)
	modeErr := file.Chmod(info.Mode().Perm())
	if err := errors.Join(copyErr, modeErr, file.Close()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func startUpdateHelper(target, staged, digest string, portable bool) (io.WriteCloser, error) {
	pattern := updateHelperPrefix + "*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	helper, err := copyExecutable(target, pattern)
	if err != nil {
		return nil, err
	}
	args := []string{applyUpdateFlag, target, staged, digest}
	if portable {
		args = append(args, "--portable")
	}
	cmd := exec.Command(helper, args...)
	hideWorkerWindow(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		_ = os.Remove(helper)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = input.Close()
		_ = os.Remove(helper)
		return nil, err
	}
	_ = cmd.Process.Release()
	return input, nil
}

func runUpdateHelper(args []string) error {
	if len(args) < 3 {
		return errors.New("invalid update command")
	}
	helper, err := os.Executable()
	target, staged, digest := args[0], args[1], args[2]
	if err != nil || !strings.HasPrefix(filepath.Base(helper), updateHelperPrefix) ||
		filepath.Dir(helper) != filepath.Dir(target) || filepath.Dir(staged) != filepath.Dir(target) ||
		!strings.HasPrefix(filepath.Base(staged), ".wireproxy-update-") || !validDigest(digest) {
		return errors.New("invalid update paths or digest")
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	return installUpdate(target, staged, digest, func(failed bool) error {
		return restartAfterUpdate(target, helper, slices.Contains(args[3:], "--portable"), failed)
	})
}

func installUpdate(target, staged, digest string, restart func(failed bool) error) error {
	defer os.Remove(staged)
	err := replaceExecutable(target, staged, digest, restart)
	if err != nil {
		// Reopen the previous executable with a visible error if replacement failed.
		return errors.Join(err, restart(true))
	}
	return nil
}

func replaceExecutable(target, staged, digest string, restart func(bool) error) error {
	file, err := os.Open(staged)
	if err != nil {
		return err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(hash, io.LimitReader(file, maxUpdateSize+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	if size < 1 || size > maxUpdateSize || !strings.EqualFold("sha256:"+hex.EncodeToString(hash.Sum(nil)), digest) {
		return errors.New("staged binary failed verification")
	}
	backup, err := copyExecutable(target, ".wireproxy-previous-*")
	if err != nil {
		return err
	}
	defer func() {
		if backup != "" {
			_ = os.Remove(backup)
		}
	}()
	// Windows can hold the old executable open for a short time while the UI exits.
	if err := replaceWithRetry(staged, target); err != nil {
		return err
	}
	if err := restart(false); err != nil {
		if restoreErr := replaceWithRetry(backup, target); restoreErr != nil {
			saved := backup
			backup = ""
			return fmt.Errorf("restore previous executable from %s: %w", saved, restoreErr)
		}
		return err
	}
	return nil
}

func replaceWithRetry(source, target string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Rename(source, target)
		if err == nil || runtime.GOOS != "windows" || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func restartAfterUpdate(target, helper string, portable, failed bool) error {
	args := []string{"--cleanup-update", helper}
	if portable {
		args = append(args, "--portable")
	}
	if failed {
		args = append(args, "--update-error")
	}
	cmd := exec.Command(target, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func cleanUpdateHelper(executable, helper string) {
	if filepath.Dir(helper) != filepath.Dir(executable) || !strings.HasPrefix(filepath.Base(helper), updateHelperPrefix) {
		return
	}
	for range 50 {
		if err := os.Remove(helper); err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
