# LLM API Uptime Monitor

A lightweight tool for monitoring the uptime and stability of LLM API providers (OpenAI Compatible & Anthropic).

## Features

- **Multi-provider support**: Monitor OpenAI Compatible and Anthropic APIs
- **Dual interface**: Interactive TUI (Terminal UI) and Web UI
- **Configurable probes**: Set interval, timeout, and concurrency
- **Detailed logging**: Track errors, request IDs, and timestamps
- **Availability stats**: Calculate uptime by hour/day
- **CSV export**: Generate availability reports
- **SQLite storage**: Balanced speed and size
- **Auto-cleanup**: Configurable data retention

## Installation

```bash
go build -o llm-api-uptime .
```

## Usage

### TUI Mode (Default)

```bash
./llm-api-uptime
```

### Web Mode

```bash
WEB_ENABLED=true ./llm-api-uptime -mode=web
```

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env`:

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

| Key | Action |
|-----|--------|
| `1-5` | Switch pages |
| `t` | Trigger probe (on Dashboard) |
| `q` / `Ctrl+C` | Quit |

## Web UI

When enabled, access at `http://localhost:8080`

### Pages

- **Dashboard**: Overview and recent activity
- **Providers**: Manage API providers
- **Models**: Configure probe targets
- **Statistics**: View uptime stats and export CSV

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
| `GET` | `/api/export/csv` | Export CSV report |
| `POST` | `/api/probe/trigger` | Trigger manual probe |

## Error Tracking

The system tracks:

- HTTP errors (4xx, 5xx)
- Empty responses
- Timeouts
- Network errors
- API error codes and messages
- Request IDs (from response body or `x-request-id` header)

## CSV Report Format

```csv
Provider,Model,Time Range,Total Probes,Success,Error,Timeout,Empty Response,Success Rate (%),Avg Latency (ms),Downtime Periods
ProviderA,gpt-4,2024-01-01~2024-01-07,1000,980,15,3,2,98.0,234,"2024-01-03 14:00~14:30; 2024-01-05 09:15~09:45"
```

## License

MIT
