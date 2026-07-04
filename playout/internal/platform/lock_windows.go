//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

type instanceLock struct {
	path string
	f    *os.File
}

func newInstanceLock(path string) InstanceLock {
	return &instanceLock{path: path}
}

// Acquire creates the lock file with exclusive access using O_EXCL.
// Windows does not support flock; exclusive file creation is used instead.
func (l *instanceLock) Acquire() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("instance lock: mkdir: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("instance lock: engine already running (lock %s)", l.path)
		}
		return fmt.Errorf("instance lock: open %s: %w", l.path, err)
	}

	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	l.f = f
	return nil
}

// Release closes and removes the lock file.
func (l *instanceLock) Release() error {
	if l.f == nil {
		return nil
	}
	_ = l.f.Close()
	_ = os.Remove(l.path)
	l.f = nil
	return nil
}
