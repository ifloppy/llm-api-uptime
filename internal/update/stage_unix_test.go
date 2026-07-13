//go:build linux || darwin

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStageReplacesConfiguredExecutable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "llm-api-uptime")
	oldBinary := []byte("#!/bin/sh\necho v1.0.0\n")
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	newBinary, err := os.ReadFile(testExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, oldBinary, 0o751); err != nil {
		t.Fatal(err)
	}
	assetName := AssetName("v1.1.0", runtime.GOOS, runtime.GOARCH)
	digest := sha256.Sum256(newBinary)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case latestReleasePath:
			fmt.Fprintf(response, `{"tag_name":"v1.1.0","html_url":%q,"published_at":"2026-07-01T12:00:00Z","assets":[{"name":"checksums.txt","browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
				server.URL+"/release", server.URL+"/checksums.txt", assetName, server.URL+"/binary")
		case "/checksums.txt":
			fmt.Fprintf(response, "%s  %s\n", hex.EncodeToString(digest[:]), assetName)
		case "/binary":
			_, _ = response.Write(newBinary)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	checker := NewChecker(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCurrentVersion("v1.0.0"),
		WithExecutablePath(executable),
	)
	status := checker.Stage(context.Background())
	if status.State != StateRestartRequired {
		t.Fatalf("Stage() = %#v", status)
	}
	if status.BackupPath != executable+".old" {
		t.Errorf("BackupPath = %q", status.BackupPath)
	}
	gotNew, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotNew) != string(newBinary) {
		t.Fatalf("installed binary = %q", gotNew)
	}
	gotOld, err := os.ReadFile(executable + ".old")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOld) != string(oldBinary) {
		t.Fatalf("backup binary = %q", gotOld)
	}
	info, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Fatalf("installed mode = %o", info.Mode().Perm())
	}
}

func TestStageVerificationFailureLeavesExecutableUntouched(t *testing.T) {
	tests := []struct {
		name             string
		binary           []byte
		checksumContents func(string, [sha256.Size]byte) string
	}{
		{
			name:   "checksum mismatch",
			binary: []byte("#!/bin/sh\necho v1.1.0\n"),
			checksumContents: func(assetName string, _ [sha256.Size]byte) string {
				return fmt.Sprintf("%064x  %s\n", 0, assetName)
			},
		},
		{
			name:   "invalid executable format",
			binary: []byte("not-an-executable"),
			checksumContents: func(assetName string, digest [sha256.Size]byte) string {
				return fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), assetName)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			executable := filepath.Join(directory, "llm-api-uptime")
			oldBinary := []byte("#!/bin/sh\necho v1.0.0\n")
			if err := os.WriteFile(executable, oldBinary, 0o755); err != nil {
				t.Fatal(err)
			}
			assetName := AssetName("v1.1.0", runtime.GOOS, runtime.GOARCH)
			digest := sha256.Sum256(test.binary)

			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case latestReleasePath:
					fmt.Fprintf(response, `{"tag_name":"v1.1.0","html_url":%q,"assets":[{"name":"checksums.txt","browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
						server.URL+"/release", server.URL+"/checksums.txt", assetName, server.URL+"/binary")
				case "/checksums.txt":
					_, _ = response.Write([]byte(test.checksumContents(assetName, digest)))
				case "/binary":
					_, _ = response.Write(test.binary)
				default:
					http.NotFound(response, request)
				}
			}))
			defer server.Close()

			checker := NewChecker(
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithCurrentVersion("v1.0.0"),
				WithExecutablePath(executable),
			)
			status := checker.Stage(context.Background())
			if status.State != StateError || status.Error == "" {
				t.Fatalf("Stage() = %#v, want error", status)
			}
			got, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(oldBinary) {
				t.Fatalf("executable changed to %q", got)
			}
			if _, err := os.Lstat(executable + ".old"); !os.IsNotExist(err) {
				t.Fatalf("backup exists after failed verification: %v", err)
			}
		})
	}
}

func TestInspectExecutableRejectsUnsafeFiles(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(directory, "link")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if _, _, err := inspectExecutable(symlink); err == nil {
		t.Fatal("inspectExecutable accepted a symlink")
	}
	if _, _, err := inspectExecutable(directory); err == nil {
		t.Fatal("inspectExecutable accepted a directory")
	}
}

func TestValidateStagedBinaryRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(path, []byte("not-an-executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateStagedBinary(path); err == nil {
		t.Fatal("validateStagedBinary accepted a malformed file")
	}
}

func TestReplaceExecutableKeepsExistingBackup(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "llm-api-uptime")
	staged := filepath.Join(directory, "staged")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable+".old", []byte("older"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	backup, err := replaceExecutable(executable, staged)
	if err != nil {
		t.Fatal(err)
	}
	if backup != executable+".old.1" {
		t.Fatalf("backup = %q, want %q", backup, executable+".old.1")
	}
	if got, err := os.ReadFile(executable + ".old"); err != nil || string(got) != "older" {
		t.Fatalf("existing backup changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(backup); err != nil || string(got) != "current" {
		t.Fatalf("new backup = %q, %v", got, err)
	}
}
