package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/tool/offlinepaymentbackfill"
	_ "github.com/lib/pq"
)

const offlinePaymentBackfillConfirmToken = "offline-paid-backfill-20260716"

type backfillOptions struct {
	execute  bool
	operator string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseBackfillArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "参数错误: %v\n", err)
		return 2
	}

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		fmt.Fprintf(stderr, "读取配置失败: %v\n", err)
		return 1
	}

	db, err := sql.Open("postgres", cfg.Database.DSNWithTimezone(cfg.Timezone))
	if err != nil {
		fmt.Fprintf(stderr, "打开数据库连接失败: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	applyBackfillDBPoolSettings(db, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := offlinepaymentbackfill.RunOfflinePaymentBackfill(ctx, db, opts.operator, opts.execute)
	if err != nil {
		fmt.Fprintf(stderr, "补录校验失败: %v\n", err)
		return 1
	}

	printBackfillResult(stdout, opts, result)
	return 0
}

func parseBackfillArgs(args []string) (backfillOptions, error) {
	fs := flag.NewFlagSet("offline-payment-backfill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	dryRun := fs.Bool("dry-run", false, "只执行校验并回滚，默认模式")
	execute := fs.Bool("execute", false, "执行固定五笔私下付款历史补录")
	confirm := fs.String("confirm", "", "执行写入时必须提供的确认 token")
	operator := fs.String("operator", "", "补录执行人，例如 admin:13")

	if err := fs.Parse(args); err != nil {
		return backfillOptions{}, err
	}
	if *dryRun && *execute {
		return backfillOptions{}, fmt.Errorf("--dry-run and --execute cannot be used together")
	}

	normalizedOperator := strings.TrimSpace(*operator)
	if normalizedOperator == "" {
		return backfillOptions{}, fmt.Errorf("operator is required")
	}
	if *execute && strings.TrimSpace(*confirm) != offlinePaymentBackfillConfirmToken {
		return backfillOptions{}, fmt.Errorf("confirmation token %q is required for --execute", offlinePaymentBackfillConfirmToken)
	}

	return backfillOptions{
		execute:  *execute,
		operator: normalizedOperator,
	}, nil
}

func applyBackfillDBPoolSettings(db *sql.DB, cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.Database.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetimeMinutes > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetimeMinutes) * time.Minute)
	}
	if cfg.Database.ConnMaxIdleTimeMinutes > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.Database.ConnMaxIdleTimeMinutes) * time.Minute)
	}
}

func printBackfillResult(w io.Writer, opts backfillOptions, result offlinepaymentbackfill.OfflinePaymentBackfillResult) {
	mode := "dry-run"
	if opts.execute {
		mode = "execute"
	}
	fmt.Fprintf(w, "mode=%s\n", mode)
	fmt.Fprintf(w, "operator=%s\n", opts.operator)
	fmt.Fprintf(w, "planned=%d\n", result.Planned)
	fmt.Fprintf(w, "created=%d\n", result.Created)
	fmt.Fprintf(w, "existing=%d\n", result.Existing)
	fmt.Fprintf(w, "noop=%t\n", result.Noop)
	fmt.Fprintln(w, "total_amount_cny=145.00")
}
