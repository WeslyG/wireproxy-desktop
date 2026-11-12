package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The lock is shared by every desktop copy for the current user, including
// portable copies. The OS releases it when the file closes or the process exits.
func acquireDesktopInstance(restarting bool) (*os.File, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(configDir, "Wireproxy Desktop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create desktop lock directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open desktop lock: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		locked, err := tryDesktopLock(file)
		if locked {
			return file, nil
		}
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("lock desktop instance: %w", err)
		}
		// The update helper can relaunch while the old process is closing.
		if !restarting || !time.Now().Before(deadline) {
			_ = file.Close()
			return nil, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}
