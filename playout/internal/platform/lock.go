// Package platform provides OS-level utilities: instance locking, signal handling.
package platform

import "fmt"

// InstanceLock prevents multiple engine instances with the same ID from running.
// Call Acquire on startup and Release on shutdown.
type InstanceLock interface {
	Acquire() error
	Release() error
}

// NewInstanceLock returns a platform-specific lock backed by a PID file at path.
// Conventional path: os.TempDir() + "/playout-<engine-id>.lock"
func NewInstanceLock(path string) InstanceLock {
	return newInstanceLock(path)
}

// LockPath returns the canonical lock file path for the given engine ID.
func LockPath(engineID string) string {
	return fmt.Sprintf("/tmp/playout-%s.lock", engineID)
}
