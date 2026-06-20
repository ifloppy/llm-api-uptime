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

## 📦 安装

```bash
go build -o llm-api-uptime .
```

## 🚀 使用

程序同时启动 TUI 和 Web 服务器（如果启用）：

```bash
# 仅 TUI（WEB_ENABLED=false 或未设置）
./llm-api-uptime

# TUI + Web（.env 中设置 WEB_ENABLED=true）
./llm-api-uptime
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
| `LOG_LEVEL` | `info` | 日志级别：debug, info, warn, error |

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
| **仪表盘** | 引擎状态、Web 状态、上次探测时间、活跃概览 |
| **服务商** | 增删改查 API 服务商（含 MaxTokens 配置） |
| **模型** | 添加/删除探测目标，从 API 一键获取模型列表 |
| **统计** | 可用率、TPS、导出 CSV、清空统计 |

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

## 📝 License

MIT License - 详见 [LICENSE](LICENSE) 文件。
