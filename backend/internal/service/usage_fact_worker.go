package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type UsageFactWorkerConfig struct {
	BatchSize    int
	PollInterval time.Duration
	TaskTimeout  time.Duration
}

type UsageFactSettler interface {
	Settle(ctx context.Context, fact UsageFact) error
}

type UsageFactWorker struct {
	repo      UsageFactRepository
	settler   UsageFactSettler
	cfg       UsageFactWorkerConfig
	stopCh    chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
	wg        sync.WaitGroup
}

func NewUsageFactWorker(repo UsageFactRepository, settler UsageFactSettler, cfg UsageFactWorkerConfig) *UsageFactWorker {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = 10 * time.Second
	}
	return &UsageFactWorker{
		repo:    repo,
		settler: settler,
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
}

func (w *UsageFactWorker) Start() {
	if w == nil || w.repo == nil || w.settler == nil {
		return
	}
	w.startOnce.Do(func() {
		w.wg.Add(1)
		go w.run()
	})
}

func (w *UsageFactWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
		w.wg.Wait()
	})
}

func (w *UsageFactWorker) run() {
	defer w.wg.Done()
	w.runOnce(context.Background())
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.runOnce(context.Background())
		}
	}
}

func (w *UsageFactWorker) runOnce(ctx context.Context) {
	if w == nil || w.repo == nil || w.settler == nil {
		return
	}
	now := time.Now()
	leaseUntil := now.Add(w.cfg.TaskTimeout + w.cfg.PollInterval + 5*time.Second)
	facts, err := w.repo.ClaimPending(ctx, w.cfg.BatchSize, now, leaseUntil)
	if err != nil {
		slog.Error("claim usage facts failed", "error", err)
		return
	}
	for _, fact := range facts {
		taskCtx, cancel := context.WithTimeout(ctx, w.cfg.TaskTimeout)
		err := w.settler.Settle(taskCtx, fact)
		cancel()
		if err == nil {
			continue
		}
		retryAt := time.Now().Add(usageFactRetryBackoff(fact.AttemptCount))
		if markErr := w.repo.MarkRetry(ctx, fact.ID, err.Error(), retryAt); markErr != nil {
			slog.Error("mark usage fact retry failed", "fact_id", fact.ID, "error", markErr, "settlement_error", err)
		}
	}
}

func usageFactRetryBackoff(attemptCount int) time.Duration {
	if attemptCount < 0 {
		attemptCount = 0
	}
	if attemptCount > 8 {
		attemptCount = 8
	}
	return time.Duration(1<<attemptCount) * time.Second
}
