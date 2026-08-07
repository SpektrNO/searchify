package mcp

import (
	"log/slog"
	"time"
)

func elapsedMs(start time.Time) int {
	ms := int(time.Since(start) / time.Millisecond)
	if ms < 0 {
		return 0
	}
	return ms
}

func logToolTiming(tool string, durationMs int, attrs ...any) {
	args := []any{"tool", tool, "duration_ms", durationMs}
	args = append(args, attrs...)
	slog.Info("tool complete", args...)
}
