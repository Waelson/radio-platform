package queue_test

import (
	"os"
	"time"
)

// nowMs returns the current time in Unix milliseconds.
func nowMs() int64 {
	return time.Now().UnixMilli()
}

// nowPlus returns a deadline (Unix ms) that is offsetMs milliseconds from now.
func nowPlus(offsetMs int64) int64 {
	return nowMs() + offsetMs
}

// sleepMs sleeps for the given number of milliseconds.
func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// writeFileBytes writes data to path, creating or truncating the file.
func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
