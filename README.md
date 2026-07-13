# LLM API Uptime Monitor

> [中文文档](README.zh-CN.md)

A lightweight tool for monitoring the uptime and stability of LLM API providers (OpenAI Compatible & Anthropic).

> **AI Notice**: This project is developed with assistance from AI tools.

## Features

- **Multi-provider support**: Monitor OpenAI Compatible and Anthropic APIs
- **Dual interface**: Interactive TUI (Terminal UI) and Web UI running simultaneously
- **Configurable probes**: Set interval, timeout, and concurrency
- **Per-provider MaxTokens**: Configure different max tokens for each provider
- **Content validation**: Detect empty responses (including reasoning models like DeepSeek)
- **TPS tracking**: Track tokens per second for performance monitoring
- **Detailed logging**: Track errors, request IDs, and timestamps
- **Availability stats**: Calculate uptime by hour/day
- **CSV export**: Generate availability reports with TPS data
- **Clear statistics**: Reset stats with confirmation (TUI and Web)
- **SQLite storage**: Balanced speed and size with WAL mode
- **Auto-cleanup**: Configurable data retention
- **.env support**: Automatic loading of environment variables
- **Authentication**: Optional password protection for Web UI
- **Update checks**: Discover new GitHub releases and optionally stage a verified replacement binary

## Installation

Download the binary for your operating system and architecture from [GitHub Releases](https://github.com/ifloppy/llm-api-uptime/releases), verify it against `checksums.txt`, and make it executable on Linux or macOS:

```bash
sha256sum -c checksums.txt --ignore-missing
chmod +x llm-api-uptime_v1.2.3_linux_amd64
mv llm-api-uptime_v1.2.3_linux_amd64 llm-api-uptime
```

Windows release binaries use the `.exe` suffix. Release assets are available for Linux, macOS (`darwin`), and Windows on `amd64` and `arm64`.

To build from source instead:

```bash
make build
```

## Usage

The application starts both TUI and Web server (if enabled) simultaneously:

```bash
# Start with TUI only (WEB_ENABLED=false or not set)
./llm-api-uptime

# Start with TUI + Web server (WEB_ENABLED=true in .env)
./llm-api-uptime

# Print version, commit, and build date
./llm-api-uptime --version
```

## Configuration

All configuration is via environment variables. Create a `.env` file:

```bash
cp .env.example .env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `PROBE_INTERVAL` | `5m` | Probe interval (e.g., `5m`, `1h`) |
| `PROBE_TIMEOUT` | `30s` | Single request timeout |
| `PROBE_CONCURRENCY` | `3` | Max concurrent probes per provider |
| `DB_PATH` | `./data/uptime.db` | SQLite database path |
| `DATA_RETENTION` | `720h` | Data retention period (30 days) |
| `WEB_ENABLED` | `false` | Enable web server |
| `WEB_PORT` | `8080` | Web server port |
| `WEB_PUBLIC` | `false` | Bind to 0.0.0.0 (public access) |
| `WEB_PASSWORD` | _(empty)_ | Access password (empty = no auth) |
| `WEB_GUEST_ENABLED` | `false` | Enable read-only guest access without sensitive data |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `UPDATE_CHECK_ENABLED` | `true` | Check GitHub Releases for newer versions |
| `UPDATE_CHECK_INTERVAL` | `24h` | Interval between automatic update checks |
| `UPDATE_AUTO_STAGE` | `true` | Download and stage a supported update for activation on restart |

## TUI Controls

### Global
| Key | Action |
|-----|--------|
| `1-5` | Switch pages |
| `Ctrl+C` | Quit |

### Dashboard
| Key | Action |
|-----|--------|
| `t` | Trigger probe |
| `w` | Toggle web server |

### Providers / Models
| Key | Action |
|-----|--------|
| `↑/↓` | Navigate |
| `a` | Add |
| `e` | Edit (Providers only) |
| `d` | Delete |
| `f` | Fetch models (Models only) |

### Statistics
| Key | Action |
|-----|--------|
| `↑/↓` | Navigate |
| `h` | Last 24 hours |
| `w` | Last 7 days |
| `m` | Last 30 days |
| `e` | Export CSV |
| `c` | Clear statistics (with confirmation) |

## Web UI

When enabled, access at `http://localhost:8080` (or your configured port).

### Pages

- **Dashboard**: Provider/model operational status, engine and access state, last probe time, provider/model counts, current availability and performance, and manual probe trigger
- **Providers**: CRUD operations for API providers (with MaxTokens config)
- **Models**: Add/delete probe targets, fetch from API
- **Statistics**: Filter summary data by 24 hours/7 days/30 days; inspect the latest 30-day charts and detailed uptime, TPS, and TTFT data; export CSV; or clear statistics with confirmation

The dashboard's **Copy current status** action copies the current service summary to the clipboard for sharing. Destructive statistics and history actions require confirmation and are hidden in read-only guest mode.

### Updates

Update checks compare the running version with GitHub Releases. The dashboard shows the current and latest versions, update status, and release notes when available. When auto-stage is enabled, the updater may download the matching release binary, verify its SHA-256 value from `checksums.txt`, statically validate its executable format and architecture without running it, and stage it; the dashboard then offers **Restart to update**.

Automatic staging is supported only on Linux and macOS (`darwin`) for `amd64` and `arm64`. Windows binaries are published for `amd64` and `arm64`, but Windows and all other platforms require a manual install.

The Web UI restart action is available only when `WEB_PASSWORD` is configured and the request is authenticated. Without a password, the dashboard still reports that an update is staged, but the process must be restarted manually.

SHA-256 detects accidental corruption and a mismatch between an asset and the downloaded checksum file. Because both files come from the same GitHub release and are not signed, checksums alone do not protect against compromise of the repository or release publisher. This project intentionally does not publish signatures.

### Authentication

If `WEB_PASSWORD` is set:
- Login page at `/login.html`
- Token stored in localStorage
- All API requests require `Authorization: Bearer <token>` header

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/status` | System status |
| `GET` | `/api/providers` | List providers |
| `POST` | `/api/providers` | Add provider |
| `PUT` | `/api/providers/{id}` | Update provider |
| `DELETE` | `/api/providers/{id}` | Delete provider |
| `GET` | `/api/providers/{id}/models` | Fetch models from API |
| `GET` | `/api/probes` | List probes |
| `POST` | `/api/probes` | Add probe |
| `DELETE` | `/api/probes/{id}` | Delete probe |
| `GET` | `/api/stats` | Get statistics |
| `DELETE` | `/api/stats` | Clear all statistics |
| `GET` | `/api/export/csv` | Export CSV report |
| `POST` | `/api/probe/trigger` | Trigger manual probe |
| `POST` | `/api/login` | Authenticate |

## Error Tracking

The system tracks multiple status types:

- **success**: Valid response with content
- **error**: HTTP errors, API errors, network errors
- **timeout**: Request timeout
- **empty_response**: HTTP 200 but empty body
- **empty_content**: Response with empty content (validates both `content` and `reasoning_content`)

## TPS Tracking

Tokens Per Second is calculated as:
```
TPS = completion_tokens / (latency_ms / 1000)
```

For reasoning models (like DeepSeek), `completion_tokens` includes reasoning tokens.

## CSV Report Format

```csv
Provider,Model,Time Range,Total Probes,Success,Error,Timeout,Empty Response,Empty Content,Success Rate (%),Avg Latency (ms),Avg TPS,Downtime Periods
ProviderA,gpt-4,2024-01-01 ~ 2024-01-07,1000,980,15,3,2,0,98.0,234,45.20,"2024-01-03 14:00 ~ 2024-01-03 14:30; 2024-01-05 09:15 ~ 2024-01-05 09:45"
```

## CodeGraph

CodeGraph is optional local code intelligence for contributors. Install its CLI, then initialize this checkout once:

```bash
codegraph init
codegraph status
codegraph explore "how does a probe result reach the dashboard?"
```

The file watcher normally keeps the graph current. Use `codegraph sync` after a branch switch or bulk change if needed, `codegraph impact <symbol>` before changing shared code, and `codegraph affected <files...>` to identify relevant tests. The generated `.codegraph/` index is ignored and must never be committed.

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

[中文文档](README.zh-CN.md)
