# 首次用 GHCR 镜像部署生产

- 时间：2026-09-05 20:08（+09）
- 结果：生产已从本地构建的 `sub2api:local` 切换到 GHCR 镜像，实测计费正确
- 部署内容：本次会话合并的 4 个 PR（GPT-6 定价、别名兜底修复、退款事务、CI workflow）

---

## 1. 部署前踩的一个坑：迁移数量对不上

查生产 `schema_migrations` 发现**已应用 284 条，而仓库只有 258 个迁移文件**，
且生产有 `194-206`（channel_monitor_v2）、`217-226`（分组视频/音频/搜索定价、
group_model_pricing、用量 rollups）等 26 条仓库里没有的迁移。

据此我**两次**得出错误结论：

1. 「生产跑的不是这个仓库的代码」
2. 「部署本仓库会是降级，应该先合并落后 1095 个提交的上游」

甚至已经开始 merge upstream 并产生了 47 个冲突。

**真相**：`/opt/sub2api/src` 有 **258 个**迁移文件（与仓库一致），
含 fork 独有的 `212_`、不含上游的 `226_`。**生产跑的就是这个 fork。**

那 26 条多余的已应用记录，是**数据库从旧实例 pg_restore 恢复时带来的历史痕迹**——
旧实例曾跑过更新的上游构建。迁移运行器只执行「文件存在但未应用」的，
多余的行不影响启动。**生产已健康运行 20 小时本身就是证明。**

### 判据

| 检查 | 结论 |
| --- | --- |
| 仓库有、生产没应用的迁移 | 0 条 → 代码所需迁移全部就位 |
| `/opt/sub2api/src` 的迁移文件数 | 258，与仓库一致 |
| 源码文件哈希比对 | `openai_model_alias.go` = 会话起点 `e68e9bd5a`；`payment_balance_package_refund.go` = 含退款事务改动的版本 |

**教训：`schema_migrations` 行数多于迁移文件数是正常的**（DB 比代码活得久，
经历过恢复/迁移），不能据此判断代码来源。要判断代码来源就去看源码本身。

顺带确认：会话开始时工作区里那处「会话前就存在的未提交退款改动」，
**当时已经应用在 VPS 源码上了**——所以它在部署前就已经生效。

---

## 2. 部署内容

`e68e9bd5a`（生产基线）→ `497cbda67`（已构建镜像）的代码增量：
**6 个文件，全在 `backend/internal/service/`，无迁移、无前端、无配置。**

| 文件 | 内容 |
| --- | --- |
| `pricing_service.go` | `matchOpenAIModel` 的 GPT-6 分支 + 静态价（PR #5 的真正修复） |
| `billing_service.go` | `fallbackPrices["gpt-6-astra"]` + 补全策略 |
| `openai_model_alias.go` | `gpt6` → `gpt-6` 归一化、`isOpenAIGPT6Model` |
| `payment_refund.go` / `payment_balance_package_refund.go` | 退款事务原子性（VPS 源码已有，部署不改变行为） |
| `billing_service_gpt6_test.go` | 测试，不进二进制 |

> 最新两个提交是纯文档，被 `build.yml` 的 `paths-ignore` 跳过（设计如此），
> 所以最新镜像对应 `497cbda67` 而非 `main` 的 HEAD。代码内容相同。

---

## 3. 执行步骤

GHCR 包是**公开的**，VPS 无需 `docker login` 即可拉取。

1. 备份 `docker-compose.vps.yml` 与 `.env`（带时间戳）
2. `image: sub2api:local` → `image: ghcr.io/cnyui/sub2api:${IMAGE_TAG:-main}`
3. `.env` 写入 `IMAGE_TAG=sha-<40 位 commit>`
4. **`docker compose config` 渲染检查**（重启前）：确认 image、端口绑定、
   `BILLING_FINAL_MULTIPLIER`、secrets 四项
5. `docker compose -f docker-compose.yml -f docker-compose.vps.yml up -d sub2api`
6. 等待 healthy（实测第 2 次轮询即 healthy，约 10 秒）

**此后换版本只需改 `.env` 的 `IMAGE_TAG` 再 `up -d`**，与
`deploy/vps-deploy.sh.example` 的设计一致。

---

## 4. 核验结果

| 项 | 结果 |
| --- | --- |
| 运行镜像 | `ghcr.io/cnyui/sub2api:sha-497cbda6...` |
| 容器内 `BILLING_FINAL_MULTIPLIER` | **`18`** |
| 凭证加密密钥 | 可读 |
| 端口绑定 | **`127.0.0.1:8080->8080/tcp`**（未暴露公网） |
| 迁移 | 仍 284 条，**无新应用、无 checksum 报错** |
| 启动日志 | 无 error / fatal / panic |
| 三域名 `/health` | 全部 `200` |

真实计费验证（`usage_logs` id `405436`）：

| 分项 | 值 | 核对 |
| --- | --- | --- |
| `input_cost` | `0.00861` | `861 × $10/M` ✓ |
| `cache_read_cost` | `0.007488` | `7488 × $1/M` ✓ |
| `output_cost` | `0.0008` | `16 × $50/M` ✓ |
| `total_cost` | `0.016898` | 与请求前预测**逐位一致** ✓ |
| `actual_cost` | `0.04866624` | `× 0.16 × 18` ✓ |

---

## 5. 回滚

旧镜像 `sub2api:local`（187MB，26 小时前构建）**仍在 VPS 上**，未被 prune。

```bash
cd ${OPS_DEPLOY_DIR}
cp docker-compose.vps.yml.bak.20260905-110618 docker-compose.vps.yml
docker compose -f docker-compose.yml -f docker-compose.vps.yml up -d sub2api
```

或保留新 compose、只把 `.env` 的 `IMAGE_TAG` 改成上一个正常的 `sha-` 版本。

**注意**：只 `docker image prune -f` 清理悬空层，**不要 `prune -a`**，
否则带 tag 的历史镜像会被删掉、失去回滚能力。

---

## 6. 仍未处理

- **自动部署仍需人工配置**：`VPS_*` secrets 与 VPS 上的 `deploy.sh`。
  本次是手工执行的，`build.yml` 的 deploy job 因缺 secrets 会优雅跳过。
- **`gpt-5.6-sol` 超收未修**（管理员决定保持现状）。上游也是同样的错值，
  合并上游修不掉。
- **长上下文计费仍一道没打开**。本次部署带来了 `gpt-6-astra` 的长上下文
  静态价，但账号级开关 `openai_long_context_billing_enabled` 默认 false，
  仍需逐个 OpenAI 上游账号打开才会生效。
- **本 fork 落后上游 1095 个提交**是事实（分叉于 `7e2e9ba05`，2026-08-02）。
  这不影响部署——生产跟随 fork——但意味着上游的新特性与修复不会自动获得。
  上游已有 GPT-6 Astra 支持（`ed7c8f220`）且比本 fork 实现更完整（含按模型的
  Fast/priority 倍率）。是否合并上游是独立的产品决策，本次未做，
  试合并产生 47 个冲突已 `merge --abort` 回滚。
