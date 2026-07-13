//go:build linux || darwin

// Package restart replaces the current process after a staged update.
package restart

import (
	"fmt"
	"os"
	"syscall"
)

// Supported reports whether in-process restart is available on this platform.
func Supported() bool { return true }

// Exec replaces the current process while preserving its arguments and environment.
func Exec(executable string, args, env []string) error {
	return syscall.Exec(executable, args, env)
}

// Restore atomically restores the previous executable after a failed exec.
func Restore(executable, backup string) error {
	if backup == "" {
		return fmt.Errorf("no update backup is available")
	}
	if err := os.Rename(backup, executable); err != nil {
		return fmt.Errorf("restore previous executable: %w", err)
	}
	return nil
}
