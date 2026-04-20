package runtime

import (
	"log/slog"
	"runtime/debug"
)

// Recover 统一的 panic 兜底：记录 where + error + goroutine stack。
// 用法：`defer runtime.Recover("scheduler.tick")`。
func Recover(where string) {
	r := recover()
	if r == nil {
		return
	}
	slog.Error("panic.recovered",
		"where", where,
		"err", r,
		"stack", string(debug.Stack()),
	)
}
