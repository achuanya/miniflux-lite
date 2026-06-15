// Command miniflux-lite 通过 Miniflux API 抓取指定分类下每个订阅源的最新一篇文章，
// 聚合后写入本地 JSON 文件。
//
// 程序入口仅负责装配各模块并编排流程；具体职责分散在 internal 下的各子包中。
package main

import (
	"context"
	"fmt"
	"os"

	"miniflux-lite/internal/aggregator"
	"miniflux-lite/internal/api"
	"miniflux-lite/internal/config"
	"miniflux-lite/internal/logger"
	"miniflux-lite/internal/storage"

	"go.uber.org/zap"
)

func main() {
	os.Exit(run())
}

// run 执行主流程并返回进程退出码（0 成功，非 0 失败）。
// 单独抽出便于在返回前正确执行 defer（如 logger.Sync）。
func run() int {
	// 1) 加载并校验配置。此阶段 logger 尚未就绪，错误直接打到 stderr。
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败:", err)
		return 1
	}

	// 2) 依据配置初始化日志。
	log, err := logger.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志初始化失败:", err)
		return 1
	}
	// Sync 在 stdout 上可能返回无害错误（如 Windows），此处忽略。
	defer func() { _ = log.Sync() }()

	// 3) 记录启动信息（Token 已脱敏）。
	log.Info("MiniFlux-Lite 启动",
		zap.String("api_url", cfg.APIURL),
		zap.String("api_token", cfg.MaskedToken()),
		zap.Strings("categories", cfg.Categories),
		zap.String("file_path", cfg.FilePath),
		zap.Int("max_concurrent", cfg.MaxConcurrent),
		zap.Duration("http_timeout", cfg.HTTPTimeout),
	)

	// 4) 构造 API 客户端。
	apiClient := api.New(cfg, log)

	// 5) 执行聚合。致命错误（如分类列表获取失败）直接退出。
	ctx := context.Background()
	items, err := aggregator.Aggregate(ctx, apiClient, cfg, log)
	if err != nil {
		log.Error("聚合失败", zap.Error(err))
		return 1
	}

	// 6) 写入输出文件。
	if err := storage.WriteJSON(cfg.FilePath, items, log); err != nil {
		log.Error("写入输出文件失败", zap.Error(err))
		return 1
	}

	log.Info("全部完成", zap.Int("item_count", len(items)))
	return 0
}
