//go:build linux || darwin

package update

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func supportedTarget() (bool, string) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return false, fmt.Sprintf("automatic updates are unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return true, ""
}

func autoStage(ctx context.Context, checker *Checker, release Release, assetName string) (string, error) {
	executable, mode, err := inspectExecutable(checker.executablePath)
	if err != nil {
		return "", err
	}
	checksumAsset, err := release.asset(checksumsAsset)
	if err != nil {
		return "", err
	}
	binaryAsset, err := release.asset(assetName)
	if err != nil {
		return "", err
	}

	checksums, err := checker.downloadBytes(ctx, checksumAsset.URL, checksumsLimit, requestTimeout)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	expectedHash, err := ParseChecksum(checksums, assetName)
	if err != nil {
		return "", err
	}

	temporary, err := createExclusiveTemp(filepath.Dir(executable), ".llm-api-uptime-update-")
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	actualHash, err := checker.downloadFile(ctx, binaryAsset.URL, temporary, assetLimit)
	if err != nil {
		temporary.Close()
		return "", fmt.Errorf("download update: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close staged update: %w", err)
	}
	if actualHash != expectedHash {
		return "", fmt.Errorf("SHA-256 checksum mismatch for %q", assetName)
	}
	if err := os.Chmod(temporaryPath, mode.Perm()); err != nil {
		return "", fmt.Errorf("set staged update permissions: %w", err)
	}
	if err := syncFile(temporaryPath); err != nil {
		return "", fmt.Errorf("sync staged update: %w", err)
	}
	if err := validateStagedBinary(temporaryPath); err != nil {
		return "", err
	}

	backupPath, err := replaceExecutable(executable, temporaryPath)
	if err != nil {
		return "", err
	}
	return backupPath, nil
}

func inspectExecutable(configuredPath string) (string, os.FileMode, error) {
	executable := configuredPath
	var err error
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			return "", 0, fmt.Errorf("resolve executable: %w", err)
		}
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", 0, fmt.Errorf("resolve executable path: %w", err)
	}
	originalInfo, err := os.Lstat(executable)
	if err != nil {
		return "", 0, fmt.Errorf("inspect executable: %w", err)
	}
	if originalInfo.Mode()&os.ModeSymlink != 0 {
		return "", 0, fmt.Errorf("refusing to replace symlink executable %q", executable)
	}

	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", 0, fmt.Errorf("resolve executable symlinks: %w", err)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", 0, fmt.Errorf("inspect resolved executable: %w", err)
	}
	mode := info.Mode()
	if !mode.IsRegular() {
		return "", 0, fmt.Errorf("refusing to replace non-regular executable %q", resolved)
	}
	if mode&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return "", 0, fmt.Errorf("refusing to replace setuid or setgid executable %q", resolved)
	}
	parent := filepath.Dir(resolved)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return "", 0, fmt.Errorf("inspect executable directory: %w", err)
	}
	if !parentInfo.IsDir() {
		return "", 0, fmt.Errorf("executable parent %q is not a directory", parent)
	}
	if err := unix.Access(parent, unix.W_OK|unix.X_OK); err != nil {
		return "", 0, fmt.Errorf("executable directory %q is not writable: %w", parent, err)
	}
	return resolved, mode, nil
}

func createExclusiveTemp(directory, prefix string) (*os.File, error) {
	for range 100 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		path := filepath.Join(directory, prefix+hex.EncodeToString(random[:]))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not allocate a unique staging file")
}

func (c *Checker) downloadFile(ctx context.Context, address string, destination *os.File, limit int64) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	requestContext, cancel := context.WithTimeout(ctx, assetTimeout)
	defer cancel()

	request, err := newDownloadRequest(requestContext, address)
	if err != nil {
		return digest, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return digest, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return digest, fmt.Errorf("HTTP %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if response.ContentLength > limit {
		return digest, fmt.Errorf("asset is too large: %d bytes (limit %d)", response.ContentLength, limit)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return digest, err
	}
	if written > limit {
		return digest, fmt.Errorf("asset exceeds %d-byte limit", limit)
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func newDownloadRequest(ctx context.Context, address string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "llm-api-uptime-updater")
	return request, nil
}

func syncFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func validateStagedBinary(path string) error {
	switch runtime.GOOS {
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return fmt.Errorf("staged update is not a valid ELF executable: %w", err)
		}
		defer file.Close()
		want := elf.EM_X86_64
		if runtime.GOARCH == "arm64" {
			want = elf.EM_AARCH64
		}
		if file.Machine != want {
			return fmt.Errorf("staged update architecture is %s, want %s", file.Machine, runtime.GOARCH)
		}
	case "darwin":
		file, err := macho.Open(path)
		if err != nil {
			return fmt.Errorf("staged update is not a valid Mach-O executable: %w", err)
		}
		defer file.Close()
		want := macho.CpuAmd64
		if runtime.GOARCH == "arm64" {
			want = macho.CpuArm64
		}
		if file.Cpu != want {
			return fmt.Errorf("staged update architecture is %s, want %s", file.Cpu, runtime.GOARCH)
		}
	default:
		return fmt.Errorf("automatic updates are unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

func replaceExecutable(executable, stagedPath string) (string, error) {
	backupPath := executable + ".old"
	for suffix := 0; ; suffix++ {
		if suffix > 0 {
			backupPath = fmt.Sprintf("%s.old.%d", executable, suffix)
		}
		if _, err := os.Lstat(backupPath); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return "", fmt.Errorf("inspect backup path: %w", err)
		}
	}
	if err := os.Link(executable, backupPath); err != nil {
		return "", fmt.Errorf("back up current executable: %w", err)
	}
	if err := os.Rename(stagedPath, executable); err != nil {
		if cleanupErr := os.Remove(backupPath); cleanupErr != nil {
			return "", errors.Join(fmt.Errorf("install staged executable: %w", err), fmt.Errorf("remove unused backup: %w", cleanupErr))
		}
		return "", fmt.Errorf("install staged executable (original unchanged): %w", err)
	}
	if directory, err := os.Open(filepath.Dir(executable)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}

	return backupPath, nil
}
