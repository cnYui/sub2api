# 余额购买流量包事务修复结果

## 结论

- 已修复余额购买流量包时 `user_traffic_credits_order_id_fkey` 外键失败。
- `trafficPackRepository.CreditPurchase()` 现在会检测 `context` 中的 Ent 事务：余额支付已有外层事务时复用 `tx.Client()`；普通已提交订单发货没有外层事务时，继续自行开启并提交本地 SQL 事务。
- 余额扣减、订单创建、流量额度和购买流水恢复为同一事务内同成同败。
- 未移除或延迟外键，未提前提交订单，未引入异步补偿，也未修改 `TrafficPackRepository` 服务接口。

## TDD 证据

新增 PostgreSQL 集成测试 `TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction`：

1. 创建已提交用户。
2. 开启 Ent 外层事务，在事务内创建尚未提交的余额流量包订单。
3. 使用带 `dbent.NewTxContext()` 的 context 调用真实 `CreditPurchase()`。
4. 验证事务内存在一条流量额度和一条购买流水。
5. 回滚外层事务后，验证订单、流量额度和流水在全局连接中均不存在。

修复前 RED：

```text
pq: insert or update on table "user_traffic_credits" violates foreign key constraint "user_traffic_credits_order_id_fkey"
FAIL github.com/Wei-Shaw/sub2api/internal/repository
```

修复后 GREEN：

```text
ok github.com/Wei-Shaw/sub2api/internal/repository 7.105s
```

## 修改范围

- `backend/internal/repository/traffic_pack_repo.go`
  - 新增 `withCreditPurchaseTx()`，复用已有 Ent 事务或创建本地 SQL 事务。
  - `CreditPurchase()` 改用统一 `sqlExecutor` 写额度和流水。
  - 使用 `scanSingleRow()` 兼容 `*sql.Tx` 与 Ent transaction client。
- `backend/internal/repository/traffic_pack_repo_integration_test.go`
  - 新增真实 PostgreSQL 外键与外层回滚回归测试。
- `AGENTS.md`
  - 增加本次修复定论和验证范围。

`Deduct()`、数据库 schema、migration、前端和支付 service 接口均未修改。

## 验证

以下命令全部通过：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction$'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository -run '^TestTrafficPackRepository_'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run '^TestBalancePay'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository ./internal/service
git diff --check
```

完整相关包结果：`internal/repository` 通过，`internal/service` 通过（89.303s）。

## 运行态范围

- 未补写失败的临时订单 `158/159/160`。
- 未修改 `1038686518@qq.com` 的余额、返利、订单、流量额度或流水。
- 未修改 Postgres、Redis、nginx 或 CLIProxyAPI 运行态。
- 未构建 Docker 镜像，未部署公网 18084。
