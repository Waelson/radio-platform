//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type instanceLock struct {
	path string
	f    *os.File
}

func newInstanceLock(path string) InstanceLock {
	return &instanceLock{path: path}
}

// Acquire creates the lock file and acquires an exclusive flock.
// If another live process already holds the lock, returns an error with its PID.
func (l *instanceLock) Acquire() error {
	// Check for stale PID file from a previous crash (no flock held).
	if pid, err := readPID(l.path); err == nil {
		if processExists(pid) {
			return fmt.Errorf("engine already running (pid %d, lock %s)", pid, l.path)
		}
		// Stale lock — remove it so we can create a fresh one.
		_ = os.Remove(l.path)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("instance lock: open %s: %w", l.path, err)
	}

	// Exclusive non-blocking flock — fails immediately if another process holds it.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return fmt.Errorf("instance lock: flock %s: engine already running", l.path)
	}

	// Write our PID so humans / monitoring can identify the process.
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())

	l.f = f
	return nil
}

// Release releases the flock and removes the lock file.
func (l *instanceLock) Release() error {
	if l.f == nil {
		return nil
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
	_ = os.Remove(l.path)
	l.f = nil
	return nil
}

// readPID reads the PID written in a lock file.
func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, errors.New("invalid pid in lock file")
	}
	return pid, nil
}

// processExists returns true if a process with the given PID is alive.
func processExists(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks process existence without sending a real signal.
	return p.Signal(syscall.Signal(0)) == nil
}
