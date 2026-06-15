# MiniFlux-Lite

一个 Go 命令行工具：通过 [Miniflux](https://miniflux.app/) API 抓取**指定分类**下每个订阅源的**最新一篇文章**，聚合后写入本地 JSON 文件，可为博客生成多分类聚合数据

## 配置

复制 `.env.example` 为 `.env` 并填写：

| 变量 | 说明 |
| --- | --- |
| `API_URL` | Miniflux 服务器地址 |
| `API_TOKEN` | Miniflux API 访问令牌 |
| `CATEGORIES` | 需要聚合的分类名称（逗号分隔） |
| `FILE_PATH` | 输出 JSON 文件路径 |
| `LOG_LEVEL` | 日志级别（DEBUG/INFO/WARN/ERROR/...） |
| `LOG_FILE_PATH` | 日志文件路径，留空输出到 stdout |
| `LOG_ENCODER` | `json`（结构化）或 `console`（易读） |
| `MAX_CONCURRENT_GOROUTINES` | 最大并发数，`1` 为顺序执行 |
| `HTTP_TIMEOUT_SECONDS` | HTTP 请求超时（秒） |
| `HTTP_USER_AGENT` | 可选，自定义 User-Agent |

## 构建与运行

```bash
cd miniflux-lite
go mod tidy
go build
./miniflux-lite
```

## 输出格式

输出为 JSON 数组，每个元素对应一个订阅源的最新文章：

```json
[
  {
    "category": "网上邻居",
    "url": "https://example.com/post",
    "title": "文章标题",
    "published_at": "2026-06-15T08:30:00Z",
    "author": "作者昵称",
    "avatar": "https://example.com/avatar.png"
  }
]
```

字段说明：

- `category`：该条目所属的 Miniflux 分类名称。
- `url`：文章原始 URL。
- `title`：文章标题。
- `published_at`：发布时间（RFC3339）。
- `author`：作者昵称（条目无作者时回退为订阅源标题）。
- `avatar`：作者头像。优先取订阅源 `description` 字段（约定用作头像链接）；为空或非 URL 时回退为 Miniflux 返回的订阅源图标（base64 data URI）
