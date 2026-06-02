# Prometheus Daily

Prometheus Daily（普罗米修斯日报）是一个轻量 AI 技术日报系统。它从固定 RSS 源和智谱 Web Search 结果中收集信息，交给 LLM 生成 Markdown 日报，并以文件形式保存到 `content/daily/`。

项目当前目标是保持 MVP 简单可靠：文件即数据、Go 标准库后端、单文件前端、生成接口认证、Docker 部署前可本地验证。

## Features

- RSS 固定源 + 智谱 Web Search API 搜索
- Tavily API 可选补充
- DeepSeek/OpenAI 兼容 LLM 生成日报
- Markdown frontmatter 校验
- 临时文件校验后原子落盘
- `content/daily/YYYY-MM-DD.md` 文件存储
- `POST /api/generate` Bearer token 认证
- 单文件前端浏览历史日报
- `/feed.xml` RSS 订阅输出

## Project Layout

```text
cmd/server/          HTTP 服务入口
internal/config/     环境配置
internal/daily/      日期、frontmatter、Markdown 校验、文件存储
internal/search/     RSS/智谱/Tavily 搜索 provider
internal/llm/        DeepSeek/OpenAI 兼容客户端
internal/generate/   生成流水线编排
internal/httpapi/    HTTP handler 和 middleware
templates/           单文件前端模板
content/daily/       日报 Markdown 源文件
```

## Requirements

- Go 1.22+
- 智谱 API key
- DeepSeek API key
- Tavily API key 可选

## Configuration

复制 `.env.example` 为 `.env`，然后填入：

```dotenv
ADMIN_TOKEN=replace-with-long-random-token
DEEPSEEK_API_KEY=
DEEPSEEK_MODEL=deepseek-v4-flash
ZHIPU_API_KEY=
TAVILY_API_KEY=
SCHEDULE_DAILY=09:00
WORKSPACE=.
PORT=8080
```

`ADMIN_TOKEN` 用于保护生成接口。不要提交 `.env`。

Docker 部署默认使用 `TZ=Asia/Shanghai`，并在每天 09:00 自动触发日报生成。如果当天日报已存在，调度任务会跳过。

## Local Development

运行测试：

```bash
go test ./...
```

启动服务：

```bash
make run
```

打开：

```text
http://localhost:8080
```

生成日报：

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  -X POST http://localhost:8080/api/generate
```

## Deployment

服务器部署推荐在仓库目录执行：

```bash
./deploy.sh
```

脚本会执行 `git pull --ff-only`、`docker compose up -d --build`，并检查 `/health`。`.env` 只保留在服务器本地，不进入仓库。

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | 前端页面 |
| `GET` | `/about` | 项目介绍页面 |
| `GET` | `/api/daily` | 日报列表 |
| `GET` | `/api/daily/{date}/raw` | 原始 Markdown |
| `GET` | `/feed.xml` | RSS 订阅 |
| `GET` | `/rss.xml` | RSS 订阅别名 |
| `POST` | `/api/generate` | 生成日报，需要 Bearer token |
| `POST` | `/api/generate/rerun` | 重新生成今日日报，需要 Bearer token |
| `GET` | `/api/status` | 生成状态 |
| `GET` | `/health` | 健康检查 |

## Current Stop Point

当前已完成本地 Go 代码和测试。Docker 部署尚未执行，部署前需要先配置 `.env` 中的 API key。
