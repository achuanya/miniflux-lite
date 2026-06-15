// Package config 负责从 .env 文件（或系统环境变量）加载并校验运行所需的全部配置。
//
// 设计原则：禁止硬编码——所有配置项均来源于环境变量；并在程序启动阶段
// 对配置有效性进行校验，尽早暴露配置错误。
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 保存程序运行所需的全部配置项。
// 各字段已从字符串环境变量解析为合适的 Go 类型，便于后续模块直接使用。
type Config struct {
	APIURL   string // Miniflux 服务器地址
	APIToken string // Miniflux API 访问令牌

	Categories []string // 需要聚合的分类名称列表（已去除空白、去重前的有效项）
	FilePath   string   // 聚合结果 JSON 文件的输出路径

	LogLevel    string // 日志级别（debug/info/warn/error/...）
	LogFilePath string // 日志文件路径，留空表示输出到 stdout
	LogEncoder  string // 日志格式：json 或 console

	MaxConcurrent int           // 并发抓取的最大 goroutine 数量（>=1，为 1 时顺序执行）
	HTTPTimeout   time.Duration // HTTP 请求超时时间
	HTTPUserAgent string        // 自定义 User-Agent（可为空）
}

// 默认值：在对应环境变量缺失或非法时使用。
const (
	defaultLogLevel      = "info"
	defaultLogEncoder    = "console"
	defaultMaxConcurrent = 5
	defaultHTTPTimeout   = 30 * time.Second
	minTokenLength       = 16 // API Token 的最小合理长度，用于基本校验
)

// Load 读取 .env（若存在）与系统环境变量，构造并校验 Config。
//
// 当 .env 文件不存在时不视为错误——允许在容器等场景下直接通过环境变量注入配置。
// 任意必填项缺失或格式非法时返回带有明确中文说明的错误。
func Load() (*Config, error) {
	// 尝试加载当前工作目录下的 .env 文件。
	// 文件不存在是允许的（回退到系统环境变量），其他错误（如权限）才上报。
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// godotenv 在文件不存在时返回的错误可被 os.IsNotExist 识别。
		return nil, fmt.Errorf("加载 .env 文件失败: %w", err)
	}

	cfg := &Config{
		APIURL:        strings.TrimSpace(os.Getenv("API_URL")),
		APIToken:      strings.TrimSpace(os.Getenv("API_TOKEN")),
		Categories:    parseCategories(os.Getenv("CATEGORIES")),
		FilePath:      strings.TrimSpace(os.Getenv("FILE_PATH")),
		LogLevel:      getEnvDefault("LOG_LEVEL", defaultLogLevel),
		LogFilePath:   strings.TrimSpace(os.Getenv("LOG_FILE_PATH")),
		LogEncoder:    strings.ToLower(getEnvDefault("LOG_ENCODER", defaultLogEncoder)),
		MaxConcurrent: parseMaxConcurrent(os.Getenv("MAX_CONCURRENT_GOROUTINES")),
		HTTPTimeout:   parseTimeout(os.Getenv("HTTP_TIMEOUT_SECONDS")),
		HTTPUserAgent: strings.TrimSpace(os.Getenv("HTTP_USER_AGENT")),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate 校验配置项的有效性，任一项不合法立即返回错误。
func (c *Config) validate() error {
	// API_URL：必填，且为合法的 http/https URL。
	if c.APIURL == "" {
		return fmt.Errorf("配置项 API_URL 不能为空")
	}
	parsed, err := url.ParseRequestURI(c.APIURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("配置项 API_URL 不是合法的 http/https 地址: %q", c.APIURL)
	}

	// API_TOKEN：必填，且长度达到基本要求。
	if c.APIToken == "" {
		return fmt.Errorf("配置项 API_TOKEN 不能为空")
	}
	if len(c.APIToken) < minTokenLength {
		return fmt.Errorf("配置项 API_TOKEN 长度过短（至少 %d 个字符）", minTokenLength)
	}

	// CATEGORIES：去空白后至少要有一个分类。
	if len(c.Categories) == 0 {
		return fmt.Errorf("配置项 CATEGORIES 至少需要包含一个分类名称")
	}

	// FILE_PATH：必填（父目录会在写入阶段自动创建）。
	if c.FilePath == "" {
		return fmt.Errorf("配置项 FILE_PATH 不能为空")
	}

	// LOG_ENCODER：只允许 json / console。
	if c.LogEncoder != "json" && c.LogEncoder != "console" {
		return fmt.Errorf("配置项 LOG_ENCODER 仅支持 json 或 console，当前为: %q", c.LogEncoder)
	}

	return nil
}

// MaskedToken 返回脱敏后的 Token，仅用于日志输出，避免泄露完整密钥。
// 形如 "1201bb22...c91fc"。
func (c *Config) MaskedToken() string {
	const head, tail = 8, 5
	if len(c.APIToken) <= head+tail {
		return "******"
	}
	return c.APIToken[:head] + "..." + c.APIToken[len(c.APIToken)-tail:]
}

// parseCategories 将逗号分隔的分类字符串拆分为切片，去除每项首尾空白并跳过空项。
func parseCategories(raw string) []string {
	parts := strings.Split(raw, ",")
	categories := make([]string, 0, len(parts))
	for _, p := range parts {
		if name := strings.TrimSpace(p); name != "" {
			categories = append(categories, name)
		}
	}
	return categories
}

// parseMaxConcurrent 解析最大并发数；非法或小于 1 时回退为默认值（且最小为 1）。
func parseMaxConcurrent(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultMaxConcurrent
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultMaxConcurrent
	}
	return n
}

// parseTimeout 解析 HTTP 超时秒数；非法或非正数时回退为默认值。
func parseTimeout(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultHTTPTimeout
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return defaultHTTPTimeout
	}
	return time.Duration(seconds) * time.Second
}

// getEnvDefault 读取环境变量，去空白后为空则返回默认值。
func getEnvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
