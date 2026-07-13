//go:build !linux && !darwin

// Package restart provides a clear unsupported-platform result.
package restart

import (
	"fmt"
	"runtime"
)

// Supported reports whether in-process restart is available on this platform.
func Supported() bool { return false }

// Exec is unavailable because automatic staging is not supported on this platform.
func Exec(_ string, _, _ []string) error {
	return fmt.Errorf("automatic update restart is unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

// Restore is unavailable because automatic staging is unsupported.
func Restore(_, _ string) error {
	return fmt.Errorf("automatic update restore is unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}
