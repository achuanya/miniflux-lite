// Package model 定义聚合输出的数据结构。
//
// 将输出数据类型集中定义，便于各模块统一引用，也方便未来扩展输出目标
// （如写入对象存储）时复用同一份数据契约。
package model

// FeedItem 表示一个订阅源最新文章的聚合结果，对应输出 JSON 数组中的一个元素。
//
// JSON 键采用 English snake_case，作为下游消费方（如博客）依赖的数据契约。
type FeedItem struct {
	Category    string `json:"category"`     // 所处分类：该条目所属的 Miniflux 分类名称
	URL         string `json:"url"`          // 文章 URL：文章的原始链接
	Title       string `json:"title"`        // 文章标题
	PublishedAt string `json:"published_at"` // 文章时间：发布时间，RFC3339 格式
	Author      string `json:"author"`       // 作者昵称
	Avatar      string `json:"avatar"`       // 作者头像：URL 或 base64 data URI
}
