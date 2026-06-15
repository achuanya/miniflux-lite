// Package storage 负责将聚合结果持久化。
//
// 当前实现为写入本地 JSON 文件；分层独立便于未来扩展其他输出目标
// （例如腾讯 COS：新增一个实现即可，无需改动上层逻辑）。
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// WriteJSON 将 v 序列化为带缩进的 JSON 并写入 path。
//
// 行为：
//   - 自动创建父目录；
//   - 关闭 HTML 转义，避免 URL 中的 & < > 被转义为 \u00xx，同时保留中文原文；
//   - 先写临时文件再原子重命名，避免写入中途崩溃产生半截文件；
//   - 文件已存在时覆盖。
func WriteJSON(path string, v any, logger *zap.Logger) error {
	// 序列化为 JSON。使用 Encoder 以便关闭 HTML 转义。
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}

	// 确保父目录存在。
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败 %q: %w", dir, err)
	}

	// 写入临时文件，随后原子重命名为目标文件。
	tmp, err := os.CreateTemp(dir, ".feed-*.json.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	// 失败路径上清理临时文件（成功重命名后该删除为 no-op）。
	defer os.Remove(tmpName)

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("写入输出文件失败 %q: %w", path, err)
	}

	logger.Info("聚合结果已写入文件",
		zap.String("path", path),
		zap.Int("bytes", buf.Len()),
	)
	return nil
}
