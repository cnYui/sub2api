package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	balancePackageCreditLeaderLockKey = "payment:balance-package-credit:leader"
	balancePackageCreditLeaderLockTTL = 3 * time.Minute
)

// BalancePackageCreditService 每分钟扫描一次到期周额度。
type BalancePackageCreditService struct {
	packages   *BalancePackageService
	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewBalancePackageCreditService(packages *BalancePackageService, interval time.Duration) *BalancePackageCreditService {
	return &BalancePackageCreditService{packages: packages, interval: interval, stopCh: make(chan struct{}), instanceID: uuid.NewString()}
}

func (s *BalancePackageCreditService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *BalancePackageCreditService) Start() {
	if s == nil || s.packages == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *BalancePackageCreditService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *BalancePackageCreditService) runOnce() {
	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, balancePackageCreditLeaderLockKey, s.instanceID, balancePackageCreditLeaderLockTTL)
	cancel()
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	defer cancel()
	credited, err := s.packages.CreditDueBalances(ctx, time.Now().UTC())
	if err != nil {
		slog.Error("[BalancePackageCredit] failed to credit due balances", "error", err)
		return
	}
	if credited > 0 {
		slog.Info("[BalancePackageCredit] credited due balances", "count", credited)
	}
}
