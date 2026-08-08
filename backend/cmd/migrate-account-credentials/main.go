package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
)

func main() {
	apply := flag.Bool("apply", false, "写入加密后的账号凭证；未指定时只输出待迁移数量")
	batchSize := flag.Int("batch-size", 100, "每个事务处理的最大账号数")
	flag.Parse()
	if *batchSize <= 0 || *batchSize > 1000 {
		log.Fatal("batch-size 必须在 1 到 1000 之间")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	codec, err := repository.NewCredentialCodecFromConfig(cfg)
	if err != nil {
		log.Fatalf("加载账号凭证加密密钥失败: %v", err)
	}
	client, err := repository.ProvideEnt(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	result, err := migrateAccountCredentials(ctx, client, codec, *batchSize, *apply)
	if err != nil {
		log.Printf("账号凭证迁移失败: %v", err)
		os.Exit(1)
	}
	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	fmt.Printf("account_credentials_migration mode=%s scanned=%d candidates=%d migrated=%d skipped=%d\n", mode, result.scanned, result.candidates, result.migrated, result.skipped)
}

type credentialMigrationResult struct {
	scanned    int
	candidates int
	migrated   int
	skipped    int
}

type credentialMigrationAccount struct {
	id          int64
	credentials map[string]any
}

func migrateAccountCredentials(ctx context.Context, client *dbent.Client, codec *repository.CredentialCodec, batchSize int, apply bool) (credentialMigrationResult, error) {
	var result credentialMigrationResult
	var afterID int64
	for {
		tx, err := client.Tx(ctx)
		if err != nil {
			return result, err
		}
		accounts, err := loadLockedCredentialMigrationBatch(ctx, tx.Client(), afterID, batchSize)
		if err != nil {
			_ = tx.Rollback()
			return result, err
		}
		if len(accounts) == 0 {
			_ = tx.Rollback()
			return result, nil
		}
		afterID = accounts[len(accounts)-1].id
		pending := make([]credentialMigrationAccount, 0, len(accounts))
		for _, item := range accounts {
			result.scanned++
			needsMigration, err := codec.MapNeedsMigration(item.credentials)
			if err != nil {
				_ = tx.Rollback()
				return result, fmt.Errorf("verify account %d credentials: %w", item.id, err)
			}
			if needsMigration {
				pending = append(pending, item)
			} else {
				result.skipped++
			}
		}
		if !apply || len(pending) == 0 {
			_ = tx.Rollback()
			result.candidates += len(pending)
			continue
		}
		for _, item := range pending {
			stored, err := prepareAccountCredentialsMigration(codec, item.credentials)
			if err != nil {
				_ = tx.Rollback()
				return result, fmt.Errorf("prepare account %d credentials: %w", item.id, err)
			}
			if _, err := tx.Account.UpdateOneID(item.id).SetCredentials(stored).Save(ctx); err != nil {
				_ = tx.Rollback()
				return result, fmt.Errorf("persist account %d credentials: %w", item.id, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		result.candidates += len(pending)
		result.migrated += len(pending)
	}
}

func loadLockedCredentialMigrationBatch(ctx context.Context, client *dbent.Client, afterID int64, batchSize int) ([]credentialMigrationAccount, error) {
	rows, err := client.QueryContext(ctx, `
		SELECT id, credentials
		FROM accounts
		WHERE id > $1 AND deleted_at IS NULL
		ORDER BY id
		LIMIT $2
		FOR UPDATE
	`, afterID, batchSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	accounts := make([]credentialMigrationAccount, 0, batchSize)
	for rows.Next() {
		var (
			id  int64
			raw []byte
		)
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		credentials := make(map[string]any)
		if err := json.Unmarshal(raw, &credentials); err != nil {
			return nil, fmt.Errorf("decode account %d credentials: %w", id, err)
		}
		accounts = append(accounts, credentialMigrationAccount{id: id, credentials: credentials})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func prepareAccountCredentialsMigration(codec *repository.CredentialCodec, credentials map[string]any) (map[string]any, error) {
	plain, err := codec.DecryptMap(credentials)
	if err != nil {
		return nil, err
	}
	return codec.EncryptMap(plain)
}

func credentialMapNeedsMigration(credentials map[string]any) bool {
	return repository.CredentialMapNeedsMigration(credentials)
}
