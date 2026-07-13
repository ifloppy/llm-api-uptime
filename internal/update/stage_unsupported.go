//go:build !linux && !darwin

package update

import (
	"context"
	"fmt"
	"runtime"
)

func supportedTarget() (bool, string) {
	return false, fmt.Sprintf("automatic updates are unsupported on %s/%s; download the release manually", runtime.GOOS, runtime.GOARCH)
}

func autoStage(context.Context, *Checker, Release, string) (string, error) {
	return "", fmt.Errorf("automatic updates are unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}
