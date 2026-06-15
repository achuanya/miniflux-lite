// Package api 封装与 Miniflux 服务端的交互。
//
// 它以官方客户端 miniflux.app/v2/client 为主，并注入一个自定义的 *http.Client
// 以实现两点官方客户端默认不支持的能力：
//  1. 通过 http.Client.Timeout 控制请求超时（官方非 Context 方法内部硬编码 80s）；
//  2. 通过自定义 RoundTripper 覆盖 User-Agent 请求头。
//
// 此外，由于官方客户端的 Feed 结构体未包含 description 字段，而服务端
// GET /v1/categories/{id}/feeds 实际会返回该字段（被约定用作作者头像链接），
// 因此本包对 feeds 列表额外提供一个带 description 的类型化请求（见 feeds.go）。
package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"miniflux-lite/internal/config"

	"go.uber.org/zap"
	"miniflux.app/v2/client"
)

// Client 是对 Miniflux 访问能力的封装。
type Client struct {
	mf       *client.Client // 官方客户端实例
	http     *http.Client   // 自定义 HTTP 客户端（含超时与 User-Agent）
	endpoint string         // 归一化后的服务端地址，供自定义原始请求复用
	token    string         // API Token，供自定义原始请求设置鉴权头
	timeout  time.Duration  // 单次请求的超时时间
	logger   *zap.Logger
}

// uaRoundTripper 包装底层 RoundTripper，为每个请求设置自定义 User-Agent。
type uaRoundTripper struct {
	base      http.RoundTripper
	userAgent string
}

// RoundTrip 在转发请求前覆盖 User-Agent 头。
func (t *uaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.userAgent != "" {
		// 克隆请求以避免修改调用方持有的原始请求（RoundTripper 契约要求）。
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.base.RoundTrip(req)
}

// New 根据配置构造一个 api.Client。
func New(cfg *config.Config, logger *zap.Logger) *Client {
	// 自定义 HTTP 客户端：设置整体超时，并用自定义 Transport 注入 User-Agent。
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &uaRoundTripper{
			base:      http.DefaultTransport,
			userAgent: cfg.HTTPUserAgent,
		},
	}

	// 使用官方客户端，注入 API Key 与上面的自定义 HTTP 客户端。
	mf := client.NewClientWithOptions(
		cfg.APIURL,
		client.WithAPIKey(cfg.APIToken),
		client.WithHTTPClient(httpClient),
	)

	return &Client{
		mf:       mf,
		http:     httpClient,
		endpoint: normalizeEndpoint(cfg.APIURL),
		token:    cfg.APIToken,
		timeout:  cfg.HTTPTimeout,
		logger:   logger,
	}
}

// Categories 返回当前用户的全部分类。
func (c *Client) Categories(ctx context.Context) (client.Categories, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.mf.CategoriesContext(ctx)
}

// LatestEntry 返回指定订阅源最新的单篇文章；若该源没有任何条目，返回 (nil, nil)。
//
// 通过 Filter 限定只取一条、按发布时间倒序，从而拿到「最新一篇」。
// 不设置 Status 过滤，确保无论已读/未读都能取到最新条目。
func (c *Client) LatestEntry(ctx context.Context, feedID int64) (*client.Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result, err := c.mf.FeedEntriesContext(ctx, feedID, &client.Filter{
		Limit:     1,
		Offset:    0,
		Order:     "published_at",
		Direction: "desc",
	})
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Entries) == 0 {
		return nil, nil
	}
	return result.Entries[0], nil
}

// FeedIconDataURL 返回指定订阅源图标的 data URI（形如 "data:image/png;base64,..."）。
//
// 用作 description 缺失时的头像兜底来源。Miniflux 返回的 Data 形如
// "image/png;base64,..."（不含 data: 前缀），此处补全为可直接用于 <img src> 的 data URI。
func (c *Client) FeedIconDataURL(ctx context.Context, feedID int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	icon, err := c.mf.FeedIconContext(ctx, feedID)
	if err != nil {
		return "", err
	}
	if icon == nil || icon.Data == "" {
		return "", nil
	}
	if strings.HasPrefix(icon.Data, "data:") {
		return icon.Data, nil
	}
	return "data:" + icon.Data, nil
}

// normalizeEndpoint 将服务端地址归一化：去除尾部斜杠与可能的 /v1 后缀，
// 与官方客户端 NewClientWithOptions 的处理保持一致，便于拼接自定义请求路径。
func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSuffix(endpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	return endpoint
}
