package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// runUsageRecordTaskSynchronously 保证资金事务在请求或 WebSocket 轮次释放前完成。
func runUsageRecordTaskSynchronously(parent context.Context, task service.UsageRecordTask, component string) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", component),
				zap.Any("panic", recovered),
			).Error("usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}
