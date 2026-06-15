package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Feed 是本工具关心的订阅源字段集合。
//
// 之所以不直接复用官方 client.Feed，是因为官方结构体未包含 description 字段，
// 而本工具需要从该字段读取作者头像链接（服务端 API 实际会返回 description）。
type Feed struct {
	ID          int64  `json:"id"`          // 订阅源 ID
	Title       string `json:"title"`       // 订阅源标题（作者无昵称时作为回退）
	SiteURL     string `json:"site_url"`    // 站点地址
	FeedURL     string `json:"feed_url"`    // 订阅源地址
	Description string `json:"description"` // 描述字段，被约定用作作者头像链接
}

// FeedsWithDescription 获取指定分类下的全部订阅源（含 description 字段）。
//
// 由于官方客户端会在解码时丢弃 description，这里复用同一个带超时与 User-Agent 的
// *http.Client，直接对 GET /v1/categories/{id}/feeds 发起类型化请求。
func (c *Client) FeedsWithDescription(ctx context.Context, categoryID int64) ([]*Feed, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/v1/categories/%d/feeds", c.endpoint, categoryID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 feeds 请求失败: %w", err)
	}
	// 设置鉴权与内容协商头（User-Agent 由自定义 Transport 注入）。
	req.Header.Set("X-Auth-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 feeds 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 feeds 返回非预期状态码: %d", resp.StatusCode)
	}

	var feeds []*Feed
	if err := json.NewDecoder(resp.Body).Decode(&feeds); err != nil {
		return nil, fmt.Errorf("解析 feeds 响应失败: %w", err)
	}
	return feeds, nil
}
