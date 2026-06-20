# LLM API Uptime Monitor

A lightweight tool for monitoring the uptime and stability of LLM API providers (OpenAI Compatible & Anthropic).

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

## Installation

```bash
go build -o llm-api-uptime .
```

## Usage

The application starts both TUI and Web server (if enabled) simultaneously:

```bash
# Start with TUI only (WEB_ENABLED=false or not set)
./llm-api-uptime

# Start with TUI + Web server (WEB_ENABLED=true in .env)
./llm-api-uptime
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
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

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

- **Dashboard**: Overview, engine status, web server status
- **Providers**: CRUD operations for API providers (with MaxTokens config)
- **Models**: Add/delete probe targets, fetch from API
- **Statistics**: View uptime stats, TPS metrics, export CSV, clear stats

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

## License

MIT License - see [LICENSE](LICENSE) file for details.
