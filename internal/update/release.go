package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var trustedDownloadHosts = map[string]bool{
	"api.github.com":                       true,
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

const (
	githubBaseURL     = "https://api.github.com"
	latestReleasePath = "/repos/ifloppy/llm-api-uptime/releases/latest"
	checksumsAsset    = "checksums.txt"
	metadataLimit     = 1 << 20
	checksumsLimit    = 4 << 20
	assetLimit        = 256 << 20
	requestTimeout    = 20 * time.Second
	assetTimeout      = 2 * time.Minute
)

// Asset describes a downloadable GitHub release asset.
type Asset struct {
	Name string
	URL  string
}

// Release contains the stable release metadata needed by the updater.
type Release struct {
	Tag       string
	URL       string
	Published time.Time
	Assets    []Asset
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *Checker) fetchLatest(ctx context.Context) (Release, error) {
	body, err := c.downloadBytes(ctx, c.baseURL+latestReleasePath, metadataLimit, requestTimeout)
	if err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}

	var response githubRelease
	if err := json.Unmarshal(body, &response); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if response.Draft || response.Prerelease {
		return Release{}, fmt.Errorf("latest release %q is not stable", response.TagName)
	}
	if _, err := parseStableVersion(response.TagName); err != nil {
		return Release{}, fmt.Errorf("invalid release tag: %w", err)
	}
	if response.HTMLURL == "" {
		return Release{}, fmt.Errorf("latest release has no URL")
	}
	if c.baseURL == githubBaseURL {
		if err := validateTrustedURL(response.HTMLURL); err != nil {
			return Release{}, fmt.Errorf("invalid release URL: %w", err)
		}
	}
	if _, err := url.ParseRequestURI(response.HTMLURL); err != nil {
		return Release{}, fmt.Errorf("invalid release URL: %w", err)
	}

	release := Release{Tag: response.TagName, URL: response.HTMLURL}
	if response.PublishedAt != "" {
		published, err := time.Parse(time.RFC3339, response.PublishedAt)
		if err != nil {
			return Release{}, fmt.Errorf("invalid release publication time: %w", err)
		}
		release.Published = published
	}
	seen := make(map[string]struct{}, len(response.Assets))
	for _, candidate := range response.Assets {
		if candidate.Name == "" || candidate.BrowserDownloadURL == "" {
			return Release{}, fmt.Errorf("latest release contains an incomplete asset")
		}
		if _, exists := seen[candidate.Name]; exists {
			return Release{}, fmt.Errorf("latest release contains duplicate asset %q", candidate.Name)
		}
		if _, err := url.ParseRequestURI(candidate.BrowserDownloadURL); err != nil {
			return Release{}, fmt.Errorf("invalid URL for asset %q: %w", candidate.Name, err)
		}
		if c.baseURL == githubBaseURL {
			if err := validateTrustedURL(candidate.BrowserDownloadURL); err != nil {
				return Release{}, fmt.Errorf("untrusted URL for asset %q: %w", candidate.Name, err)
			}
		}
		seen[candidate.Name] = struct{}{}
		release.Assets = append(release.Assets, Asset{Name: candidate.Name, URL: candidate.BrowserDownloadURL})
	}
	return release, nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: assetTimeout,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			return validateTrustedURL(request.URL.String())
		},
	}
}

func validateTrustedURL(address string) error {
	parsed, err := url.Parse(address)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if !trustedDownloadHosts[host] {
		return fmt.Errorf("untrusted host %q", host)
	}
	return nil
}

func (r Release) asset(name string) (Asset, error) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s has no %q asset", r.Tag, name)
}

func (c *Checker) downloadBytes(ctx context.Context, address string, limit int64, timeout time.Duration) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "llm-api-uptime-updater")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("HTTP %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if response.ContentLength > limit {
		return nil, fmt.Errorf("response is too large: %d bytes (limit %d)", response.ContentLength, limit)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return body, nil
}

// ParseChecksum finds assetName in checksums.txt data and returns its SHA-256
// digest. Standard two-space and binary-marker checksum formats are accepted.
func ParseChecksum(data []byte, assetName string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	found := false
	for lineNumber, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return result, fmt.Errorf("invalid checksum line %d", lineNumber+1)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name != assetName {
			continue
		}
		if found {
			return result, fmt.Errorf("duplicate checksum for %q", assetName)
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size {
			return result, fmt.Errorf("invalid SHA-256 checksum for %q", assetName)
		}
		copy(result[:], digest)
		found = true
	}
	if !found {
		return result, fmt.Errorf("no checksum for %q", assetName)
	}
	return result, nil
}
