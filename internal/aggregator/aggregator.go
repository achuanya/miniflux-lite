// Package aggregator 负责核心聚合流程：
// 解析目标分类 -> 拉取各分类下的订阅源 -> 并发获取每个源的最新文章 ->
// 提取/加工字段 -> 汇总为 []model.FeedItem。
package aggregator

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"miniflux-lite/internal/api"
	"miniflux-lite/internal/config"
	"miniflux-lite/internal/model"

	"go.uber.org/zap"
)

// feedTask 表示一个待处理的工作项：某分类下的某个订阅源。
type feedTask struct {
	categoryTitle string    // 该订阅源所属分类名称
	feed          *api.Feed // 订阅源信息（含 description）
}

// Aggregate 执行完整聚合流程并返回结果切片。
//
// 返回的 error 仅用于「致命错误」（如无法获取分类列表）；单个订阅源的失败
// 不会导致整体失败，只会被记录并跳过。
func Aggregate(ctx context.Context, apiClient *api.Client, cfg *config.Config, logger *zap.Logger) ([]model.FeedItem, error) {
	// 1) 获取全部分类，并按配置中的名称进行匹配。
	categories, err := apiClient.Categories(ctx)
	if err != nil {
		// 分类列表无法获取属于致命错误，无法继续。
		return nil, err
	}

	// 建立「分类标题 -> ID」映射，便于按名称查找。
	titleToID := make(map[string]int64, len(categories))
	for _, c := range categories {
		titleToID[c.Title] = c.ID
	}

	// 2) 根据配置的目标分类，收集所有订阅源工作项。
	var tasks []feedTask
	for _, name := range cfg.Categories {
		categoryID, ok := titleToID[name]
		if !ok {
			// 配置中存在但服务端不存在的分类：警告并跳过。
			logger.Warn("配置的分类在 Miniflux 中不存在，已跳过", zap.String("category", name))
			continue
		}

		feeds, err := apiClient.FeedsWithDescription(ctx, categoryID)
		if err != nil {
			// 单个分类的订阅源拉取失败：记录错误并跳过该分类，不影响其余分类。
			logger.Error("获取分类下的订阅源失败，已跳过该分类",
				zap.String("category", name),
				zap.Int64("category_id", categoryID),
				zap.Error(err),
			)
			continue
		}

		logger.Info("已获取分类下的订阅源",
			zap.String("category", name),
			zap.Int("feed_count", len(feeds)),
		)
		for _, f := range feeds {
			tasks = append(tasks, feedTask{categoryTitle: name, feed: f})
		}
	}

	if len(tasks) == 0 {
		logger.Warn("没有可处理的订阅源")
		return []model.FeedItem{}, nil
	}

	// 3) 并发处理所有工作项，收集结果。
	items := process(ctx, apiClient, cfg, logger, tasks)

	// 4) 按发布时间倒序排序，保证输出确定性（最新文章在前）。
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt > items[j].PublishedAt
	})

	logger.Info("聚合完成", zap.Int("total_items", len(items)), zap.Int("total_feeds", len(tasks)))
	return items, nil
}

// process 使用信号量限制并发数处理所有工作项。
// 当 cfg.MaxConcurrent == 1 时，信号量容量为 1，天然退化为顺序执行。
func process(ctx context.Context, apiClient *api.Client, cfg *config.Config, logger *zap.Logger, tasks []feedTask) []model.FeedItem {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex // 保护 items 切片的并发写入
		items   = make([]model.FeedItem, 0, len(tasks))
		semChan = make(chan struct{}, cfg.MaxConcurrent) // 信号量：限制同时进行的 goroutine 数量
	)

	for _, task := range tasks {
		wg.Add(1)
		semChan <- struct{}{} // 获取信号量（达到上限时阻塞）

		go func(t feedTask) {
			defer wg.Done()
			defer func() { <-semChan }() // 释放信号量
			// 防御性 recover：单个订阅源处理中的意外 panic 不应影响整体程序。
			defer func() {
				if r := recover(); r != nil {
					logger.Error("处理订阅源时发生 panic，已跳过",
						zap.String("feed", t.feed.Title),
						zap.Any("recover", r),
					)
				}
			}()

			item, ok := buildItem(ctx, apiClient, logger, t)
			if !ok {
				return
			}

			mu.Lock()
			items = append(items, item)
			mu.Unlock()
		}(task)
	}

	wg.Wait()
	return items
}

// buildItem 处理单个订阅源：获取最新文章并提取/加工字段。
// 返回值 ok 为 false 表示该源应被跳过（无文章或获取失败）。
func buildItem(ctx context.Context, apiClient *api.Client, logger *zap.Logger, t feedTask) (model.FeedItem, bool) {
	// 获取最新一篇文章。
	entry, err := apiClient.LatestEntry(ctx, t.feed.ID)
	if err != nil {
		// 网络错误/超时等：记录 Error 并跳过该源（符合容错要求，不 panic、不中断整体）。
		logger.Error("获取订阅源最新文章失败，已跳过",
			zap.String("feed", t.feed.Title),
			zap.Int64("feed_id", t.feed.ID),
			zap.Error(err),
		)
		return model.FeedItem{}, false
	}
	if entry == nil {
		// 该源暂无任何文章。
		logger.Info("订阅源暂无文章，已跳过",
			zap.String("feed", t.feed.Title),
			zap.Int64("feed_id", t.feed.ID),
		)
		return model.FeedItem{}, false
	}

	item := model.FeedItem{
		Category:    t.categoryTitle,
		URL:         entry.URL,
		Title:       entry.Title,
		PublishedAt: entry.Date.Format(time.RFC3339),
		Author:      resolveAuthor(entry.Author, t.feed.Title),
		Avatar:      resolveAvatar(ctx, apiClient, logger, t.feed),
	}

	logger.Debug("已处理订阅源最新文章",
		zap.String("category", item.Category),
		zap.String("title", item.Title),
		zap.String("url", item.URL),
	)
	return item, true
}

// resolveAuthor 解析作者昵称：条目自带作者优先，为空则回退为订阅源标题。
func resolveAuthor(entryAuthor, feedTitle string) string {
	if a := strings.TrimSpace(entryAuthor); a != "" {
		return a
	}
	return feedTitle
}

// resolveAvatar 解析作者头像：
//   - 优先使用订阅源 description 字段（约定存放头像 URL），需为 http/https 链接；
//   - 否则回退为 Miniflux 返回的订阅源图标（base64 data URI）；
//   - 再失败则留空。
func resolveAvatar(ctx context.Context, apiClient *api.Client, logger *zap.Logger, feed *api.Feed) string {
	desc := strings.TrimSpace(feed.Description)
	if strings.HasPrefix(desc, "http://") || strings.HasPrefix(desc, "https://") {
		return desc
	}

	// description 为空或不是 URL，尝试图标兜底。
	dataURL, err := apiClient.FeedIconDataURL(ctx, feed.ID)
	if err != nil {
		logger.Warn("获取订阅源图标失败，头像留空",
			zap.String("feed", feed.Title),
			zap.Int64("feed_id", feed.ID),
			zap.Error(err),
		)
		return ""
	}
	return dataURL
}
