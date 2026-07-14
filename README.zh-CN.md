# LLM API Uptime Monitor

> [English Documentation](README.md)

LLM API 提供商可用性监控工具。支持 OpenAI Compatible 和 Anthropic 接口，自动定时检测 AI 服务商的可用性、延迟与 TPS。

> **AI 开发说明**：本项目基于 AI 工具辅助开发。

---

## 🎯 核心作用

| 做什么 | 怎么做 |
|--------|--------|
| 定时检测 | 每隔 N 分钟向 AI 提供商发送请求 |
| 多服务商 | 同时监控多个 API 服务商和模型 |
| 双界面 | TUI 终端界面 + Web 管理页面同时运行 |
| 结果统计 | 可用率、延迟、TPS、错误明细 |
| CSV 报告 | 导出包含故障时间段的可用性报告 |
| 轻量存储 | SQLite 存储，自动清理过期数据 |

## ✨ 特色功能

- **多提供商支持**：OpenAI Compatible + Anthropic
- **双界面并行**：TUI 和 Web 同时运行，Web 可开关
- **每提供商 MaxTokens**：独立配置每次探测的最大 token 数
- **推理模型支持**：兼容 DeepSeek 等推理模型（检查 `reasoning_content`）
- **内容检测**：防止提供商返回空内容"作弊"
- **TPS 追踪**：每秒 Token 数，监控响应速度
- **SSE 兼容**：兼容 OneAPI 等代理的 SSE 格式响应
- **BOM 兼容**：处理 UTF-8 BOM 前缀（部分中国 API 提供商）
- **详细错误记录**：记录 Request ID、错误代码、原始响应
- **可用率统计**：按小时/天计算可用率
- **CSV 导出**：含 TPS 和故障时间段的报告
- **清空统计**：支持一键清空（需二次确认）
- **SQLite 存储**：WAL 模式，速度与体积平衡
- **自动清理**：可配置数据保留时长
- **.env 支持**：自动加载环境变量文件
- **身份认证**：Web UI 支持密码保护
- **更新检查**：发现新的 GitHub Release，并可选择暂存已校验的替换程序

## 📦 安装

从 [GitHub Releases](https://github.com/ifloppy/llm-api-uptime/releases) 下载与操作系统及架构匹配的二进制文件，使用 `checksums.txt` 校验；Linux 或 macOS 还需添加执行权限：

```bash
sha256sum -c checksums.txt --ignore-missing
chmod +x llm-api-uptime_v1.2.3_linux_amd64
mv llm-api-uptime_v1.2.3_linux_amd64 llm-api-uptime
```

Windows 发布文件带 `.exe` 后缀。发布目标包括 Linux、macOS（`darwin`）和 Windows 的 `amd64`、`arm64` 架构。

也可以从源码构建：

```bash
make build
```

## 🚀 使用

程序同时启动 TUI 和 Web 服务器（如果启用）：

```bash
# 仅 TUI（WEB_ENABLED=false 或未设置）
./llm-api-uptime

# TUI + Web（.env 中设置 WEB_ENABLED=true）
./llm-api-uptime

# 显示版本、提交和构建时间
./llm-api-uptime --version
```

## ⚙️ 配置

创建 `.env` 文件：

```bash
cp .env.example .env
```

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PROBE_INTERVAL` | `5m` | 探测间隔（如 `5m`、`1h`） |
| `PROBE_TIMEOUT` | `30s` | 单次请求超时 |
| `PROBE_CONCURRENCY` | `3` | 每服务商最大并发探测数 |
| `DB_PATH` | `./data/uptime.db` | 数据库路径 |
| `DATA_RETENTION` | `720h` | 数据保留时长（30 天） |
| `WEB_ENABLED` | `false` | 启用 Web 服务器 |
| `WEB_PORT` | `8080` | Web 端口 |
| `WEB_PUBLIC` | `false` | 监听 0.0.0.0（公网访问） |
| `WEB_PASSWORD` | _(空)_ | 访问密码（空=无认证） |
| `WEB_GUEST_ENABLED` | `false` | 启用不显示敏感数据的只读访客访问 |
| `LOG_LEVEL` | `info` | 日志级别：debug, info, warn, error |
| `UPDATE_CHECK_ENABLED` | `true` | 检查 GitHub Releases 中的新版本 |
| `UPDATE_CHECK_INTERVAL` | `24h` | 自动检查更新的间隔 |
| `UPDATE_AUTO_STAGE` | `true` | 下载并暂存受支持的更新，重启后启用 |
| `UPDATE_HTTP_PROXY` | _(空)_ | 仅作用于 GitHub Release 请求的可选 HTTP 代理；留空时继承 Shell 中的 `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY`。 |

## 环境变量与代理

Shell（或 systemd 单元、进程管理器）中设置的 `HTTP_PROXY`、`HTTPS_PROXY` 与 `NO_PROXY` 会被 `llm-api-uptime` 及其在自动重启时拉起的新进程一并继承。`UPDATE_HTTP_PROXY` 仅覆盖更新器访问 GitHub Releases 时的 HTTP 客户端，不影响其它网络请求。

## ⌨️ TUI 操作

### 全局
| 按键 | 功能 |
|------|------|
| `1-5` | 切换页面 |
| `Ctrl+C` | 退出 |

### 仪表盘
| 按键 | 功能 |
|------|------|
| `t` | 手动触发探测 |
| `w` | 开关 Web 服务器 |

### 服务商 / 模型
| 按键 | 功能 |
|------|------|
| `↑/↓` | 导航 |
| `a` | 添加 |
| `e` | 编辑（服务商） |
| `d` | 删除 |
| `f` | 获取模型列表（模型页面） |

### 统计
| 按键 | 功能 |
|------|------|
| `↑/↓` | 导航 |
| `h` | 最近 24 小时 |
| `w` | 最近 7 天 |
| `m` | 最近 30 天 |
| `e` | 导出 CSV |
| `c` | 清空统计（需确认） |

## 🌐 Web UI

启用后访问 `http://localhost:8080`（或您配置的端口）。

| 页面 | 功能 |
|------|------|
| **仪表盘** | 按服务商/模型显示运行状态、引擎与访问状态、上次探测时间、服务商/模型数量、当前可用率与性能，并支持手动探测 |
| **服务商** | 增删改查 API 服务商（含 MaxTokens 配置） |
| **模型** | 添加/删除探测目标，从 API 一键获取模型列表 |
| **统计** | 按 24 小时/7 天/30 天筛选汇总数据；图表默认最近 7 天，可在工具栏切换为 30 天，并支持按模型高亮、按点 / 按天切换悬浮显示形式；查看可用率、延迟、TTFT 明细，导出 CSV 或确认后清空统计 |

仪表盘的 **Copy current status** 操作会把当前服务状态摘要复制到剪贴板，便于分享。清空统计、删除历史等破坏性操作需要确认，并且在只读访客模式下隐藏。

### 更新

更新检查会比较当前版本与 GitHub Releases。仪表盘会显示当前版本、最新版本、更新状态，并在可用时提供发布说明。启用自动暂存后，更新程序可以下载匹配的发布二进制文件，根据 `checksums.txt` 校验 SHA-256，在不运行下载文件的情况下静态检查可执行格式和架构并暂存；随后仪表盘会提供 **Restart to update** 操作。

自动暂存仅支持 Linux 和 macOS（`darwin`）的 `amd64`、`arm64` 架构。Windows 会发布 `amd64`、`arm64` 二进制文件，但 Windows 及其他平台需要手动安装。

只有配置了 `WEB_PASSWORD` 并通过认证后，Web UI 才会提供重启操作。未配置密码时，仪表盘仍会提示更新已经暂存，但需要手动重启进程。

SHA-256 可以发现意外损坏，以及发布文件与所下载校验清单之间的不一致。由于二进制文件和校验清单均来自同一个 GitHub Release 且没有签名，仅依赖校验和无法防御仓库或发布者账户被攻破。本项目按约定不发布签名。

### 认证

如果设置了 `WEB_PASSWORD`：
- 登录页面 `/login.html`
- Token 保存在 localStorage
- 所有 API 请求需要 `Authorization: Bearer <token>` 头

## 📡 API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/status` | 系统状态 |
| `GET` | `/api/providers` | 服务商列表 |
| `POST` | `/api/providers` | 添加服务商 |
| `PUT` | `/api/providers/{id}` | 更新服务商 |
| `DELETE` | `/api/providers/{id}` | 删除服务商 |
| `GET` | `/api/providers/{id}/models` | 获取模型列表 |
| `GET` | `/api/probes` | 探测目标列表 |
| `POST` | `/api/probes` | 添加探测目标 |
| `DELETE` | `/api/probes/{id}` | 删除探测目标 |
| `GET` | `/api/stats` | 获取统计 |
| `DELETE` | `/api/stats` | 清空统计 |
| `GET` | `/api/probes/{id}/results` | 探测历史明细 |
| `DELETE` | `/api/results/{id}` | 删除单条记录 |
| `GET` | `/api/export/csv` | 导出 CSV 报告 |
| `POST` | `/api/probe/trigger` | 手动触发探测 |
| `POST` | `/api/login` | 登录认证 |

## 📊 错误类型

| 状态 | 说明 |
|------|------|
| `success` | 正常响应，有内容 |
| `error` | HTTP 错误、API 错误、网络错误 |
| `timeout` | 请求超时 |
| `empty_response` | HTTP 200 但响应体为空 |
| `empty_content` | 响应有内容但为空（含推理模型检测） |

## 📈 TPS 计算

```
TPS = completion_tokens / (latency_ms / 1000)
```

对于 DeepSeek 等推理模型，`completion_tokens` 包含推理 token。

## 📄 CSV 报告格式

```csv
Provider,Model,Time Range,Total Probes,Success,Error,Timeout,Empty Response,Empty Content,Success Rate (%),Avg Latency (ms),Avg TPS,Downtime Periods
ProviderA,gpt-4,2024-01-01 ~ 2024-01-07,1000,980,15,3,2,0,98.0,234,45.20,"2024-01-03 14:00 ~ 2024-01-03 14:30; 2024-01-05 09:15 ~ 2024-01-05 09:45"
```

## CodeGraph

CodeGraph 是供贡献者使用的可选本地代码分析工具。安装 CLI 后，在当前仓库中初始化一次：

```bash
codegraph init
codegraph status
codegraph explore "how does a probe result reach the dashboard?"
```

文件监视器通常会自动保持图索引最新。切换分支或批量修改后可按需运行 `codegraph sync`；修改共享符号前运行 `codegraph impact <symbol>`；使用 `codegraph affected <files...>` 查找相关测试。生成的 `.codegraph/` 索引已被忽略，绝不能提交。

## 📝 License

MIT License - 详见 [LICENSE](LICENSE) 文件。
