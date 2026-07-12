package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ModTooL10n/engine"
)

func writeErrorReport(results []*engine.Result, batchErr error) (string, error) {
	logPath := filepath.Join(engine.OutputRoot, "..", "log.log")
	logPath = filepath.Clean(logPath)

	f, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("创建日志文件失败: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "ModTooL10n 翻译错误报告\n")
	fmt.Fprintf(f, "生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "========================================\n\n")

	if batchErr != nil {
		fmt.Fprintf(f, "全局错误: %s\n\n", batchErr.Error())
	}

	hasErrors := false

	for _, r := range results {
		if r == nil {
			continue
		}
		if r.ChunksFail == 0 {
			continue
		}
		hasErrors = true

		fmt.Fprintf(f, "Mod: %s\n", r.ModID)
		fmt.Fprintf(f, "输出: %s\n", r.OutputPath)
		fmt.Fprintf(f, "总分块: %d  成功: %d  失败: %d\n", r.ChunksTotal, r.ChunksOK, r.ChunksFail)
		for i, e := range r.Errors {
			fmt.Fprintf(f, "  错误 %d: %s\n", i+1, e)
		}
		fmt.Fprintf(f, "\n")
	}

	if !hasErrors {
		fmt.Fprintf(f, "无翻译错误\n")
	}

	return logPath, nil
}
