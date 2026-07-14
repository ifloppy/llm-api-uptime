// Package update checks and safely stages stable llm-api-uptime releases.
package update

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"llm-api-uptime/internal/buildinfo"
)

// State is the current updater lifecycle state.
type State string

const (
	StateChecking        State = "checking"
	StateUpToDate        State = "up_to_date"
	StateAvailable       State = "available"
	StateDownloading     State = "downloading"
	StateRestartRequired State = "restart_required"
	StateUnsupported     State = "unsupported"
	StateDisabled        State = "disabled"
	StateError           State = "error"
)

// Status is an immutable snapshot of checker state.
type Status struct {
	State             State     `json:"state"`
	Current           string    `json:"current"`
	Latest            string    `json:"latest,omitempty"`
	ReleaseURL        string    `json:"release_url,omitempty"`
	ReleasePublished  time.Time `json:"release_published,omitempty"`
	CheckStartedAt    time.Time `json:"check_started_at,omitempty"`
	CheckedAt         time.Time `json:"checked_at,omitempty"`
	DownloadStartedAt time.Time `json:"download_started_at,omitempty"`
	StagedAt          time.Time `json:"staged_at,omitempty"`
	BackupPath        string    `json:"backup_path,omitempty"`
	Error             string    `json:"error,omitempty"`
}

// Option customizes a Checker. Defaults target the public GitHub repository.
type Option func(*Checker)

// WithHTTPClient injects the client used for all release requests.
func WithHTTPClient(client *http.Client) Option {
	return func(checker *Checker) {
		if client != nil {
			checker.client = client
		}
	}
}

// WithBaseURL replaces the GitHub API origin. It is primarily useful in tests.
func WithBaseURL(baseURL string) Option {
	return func(checker *Checker) {
		checker.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithCurrentVersion overrides the linked build version. It is useful in tests.
func WithCurrentVersion(version string) Option {
	return func(checker *Checker) {
		checker.current = version
		checker.status.Current = version
	}
}

// WithExecutablePath directs staging at a specific executable. Callers should
// normally omit it; it exists so staging can be tested without replacing the
// running test binary.
func WithExecutablePath(path string) Option {
	return func(checker *Checker) {
		checker.executablePath = path
	}
}

// Checker checks GitHub and stages updates. Its status methods are safe for
// concurrent use, while check and staging operations are serialized.
type Checker struct {
	client         *http.Client
	baseURL        string
	current        string
	executablePath string
	interval       time.Duration
	autoStage      bool
	httpProxy      string
	stage          func(context.Context) Status

	operationMu sync.Mutex
	mu          sync.RWMutex
	status      Status
	loopMu      sync.Mutex
	loopDone    chan struct{}
}

// WithInterval sets the delay between periodic checks.
func WithInterval(interval time.Duration) Option {
	return func(checker *Checker) {
		if interval > 0 {
			checker.interval = interval
		}
	}
}

// WithAutoStage downloads an available update after each successful check.
func WithAutoStage(enabled bool) Option {
	return func(checker *Checker) {
		checker.autoStage = enabled
	}
}

// WithHTTPProxy overrides the HTTP transport used for GitHub release requests.
// The proxy is intentionally narrow: it only affects release metadata and asset
// downloads so the surrounding probe and web code paths are not reconfigured.
// A blank value clears any previously applied proxy.
func WithHTTPProxy(rawURL string) Option {
	return func(checker *Checker) {
		checker.httpProxy = strings.TrimSpace(rawURL)
	}
}

// NewChecker returns a checker configured for ifloppy/llm-api-uptime.
func NewChecker(options ...Option) *Checker {
	current := buildinfo.Current().Version
	checker := &Checker{
		client:   defaultHTTPClient(""),
		baseURL: githubBaseURL,
		current:  current,
		interval: 24 * time.Hour,
		status:  Status{State: StateChecking, Current: current},
	}
	for _, option := range options {
		option(checker)
	}
	checker.client = defaultHTTPClient(checker.httpProxy)
	checker.stage = checker.Stage
	return checker
}

// Status returns the latest checker snapshot.
func (c *Checker) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Checker) setStatus(status Status) Status {
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
	return status
}

// Start performs an immediate check and continues checking periodically until
// the context is canceled. Repeated calls while the loop is running are ignored.
func (c *Checker) Start(contexts ...context.Context) {
	ctx := operationContext(contexts)
	c.loopMu.Lock()
	if c.loopDone != nil {
		c.loopMu.Unlock()
		return
	}
	done := make(chan struct{})
	c.loopDone = done
	c.loopMu.Unlock()

	go func() {
		defer func() {
			close(done)
			c.loopMu.Lock()
			if c.loopDone == done {
				c.loopDone = nil
			}
			c.loopMu.Unlock()
		}()

		c.checkAndStage(ctx)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.checkAndStage(ctx)
			}
		}
	}()
}

func (c *Checker) checkAndStage(ctx context.Context) {
	if c.Status().State == StateRestartRequired {
		return
	}
	status := c.Check(ctx)
	if c.autoStage && status.State == StateAvailable && ctx.Err() == nil {
		if ok, notice := supportedTarget(); ok {
			c.stage(ctx)
		} else {
			status.Error = notice
			c.setStatus(status)
		}
	}
}

// Wait blocks until the periodic loop exits. It is a no-op before Start.
func (c *Checker) Wait() {
	c.loopMu.Lock()
	done := c.loopDone
	c.loopMu.Unlock()
	if done != nil {
		<-done
	}
}

// Check synchronously refreshes release status. With no context it uses
// context.Background.
func (c *Checker) Check(contexts ...context.Context) Status {
	ctx := operationContext(contexts)
	c.operationMu.Lock()
	defer c.operationMu.Unlock()

	status := Status{
		State:          StateChecking,
		Current:        c.current,
		CheckStartedAt: time.Now().UTC(),
	}
	c.setStatus(status)
	if _, err := parseStableVersion(c.current); err != nil {
		status.State = StateUnsupported
		status.CheckedAt = time.Now().UTC()
		status.Error = fmt.Sprintf("automatic updates disabled for current version %q", c.current)
		return c.setStatus(status)
	}

	release, err := c.fetchLatest(ctx)
	status.CheckedAt = time.Now().UTC()
	if err != nil {
		status.State = StateError
		status.Error = err.Error()
		return c.setStatus(status)
	}
	status.Latest = release.Tag
	status.ReleaseURL = release.URL
	status.ReleasePublished = release.Published

	comparison, err := CompareVersions(c.current, release.Tag)
	if err != nil {
		status.State = StateUnsupported
		status.Error = fmt.Sprintf("automatic updates disabled for current version %q: %v", c.current, err)
		return c.setStatus(status)
	}
	if comparison < 0 {
		status.State = StateAvailable
	} else {
		status.State = StateUpToDate
	}
	return c.setStatus(status)
}

// Stage downloads, verifies, validates, and atomically stages the latest
// release. The current process keeps running after a successful replacement.
func (c *Checker) Stage(ctx context.Context) Status {
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if _, err := parseStableVersion(c.current); err != nil {
		return c.setStatus(Status{
			State:   StateUnsupported,
			Current: c.current,
			Error:   fmt.Sprintf("automatic updates disabled for current version %q", c.current),
		})
	}

	if ok, notice := supportedTarget(); !ok {
		return c.setStatus(Status{
			State:   StateUnsupported,
			Current: c.current,
			Error:   notice,
		})
	}

	status := Status{
		State:             StateDownloading,
		Current:           c.current,
		DownloadStartedAt: time.Now().UTC(),
	}
	c.setStatus(status)

	// Refresh metadata before every staging attempt rather than trusting a
	// potentially stale prior check.
	release, err := c.fetchLatest(ctx)
	status.CheckedAt = time.Now().UTC()
	if err != nil {
		return c.stageError(status, err)
	}
	status.Latest = release.Tag
	status.ReleaseURL = release.URL
	status.ReleasePublished = release.Published
	comparison, err := CompareVersions(c.current, release.Tag)
	if err != nil {
		return c.stageError(status, err)
	}
	if comparison >= 0 {
		status.State = StateUpToDate
		return c.setStatus(status)
	}

	assetName := AssetName(release.Tag, runtime.GOOS, runtime.GOARCH)
	backupPath, err := autoStage(ctx, c, release, assetName)
	if err != nil {
		return c.stageError(status, err)
	}
	status.State = StateRestartRequired
	status.BackupPath = backupPath
	status.StagedAt = time.Now().UTC()
	return c.setStatus(status)
}

func (c *Checker) stageError(status Status, err error) Status {
	status.State = StateError
	status.Error = err.Error()
	return c.setStatus(status)
}

// Update is an alias for Stage.
func (c *Checker) Update(ctx context.Context) Status {
	return c.Stage(ctx)
}

// DownloadAndStage is an explicit alias for Stage.
func (c *Checker) DownloadAndStage(ctx context.Context) Status {
	return c.Stage(ctx)
}

func operationContext(contexts []context.Context) context.Context {
	if len(contexts) > 0 && contexts[0] != nil {
		return contexts[0]
	}
	return context.Background()
}

// AssetName returns the fixed release binary name for a target.
func AssetName(tag, goos, goarch string) string {
	name := fmt.Sprintf("llm-api-uptime_%s_%s_%s", tag, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

var defaultChecker = NewChecker()

// Start performs a check asynchronously using the package default checker.
func Start(contexts ...context.Context) *Checker {
	defaultChecker.Start(operationContext(contexts))
	return defaultChecker
}

// Check performs a check synchronously using the package default checker.
func Check(contexts ...context.Context) Status {
	return defaultChecker.Check(operationContext(contexts))
}

// CurrentStatus returns the package default checker's latest status.
func CurrentStatus() Status {
	return defaultChecker.Status()
}
