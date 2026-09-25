# 隐藏倍率 BILLING_FINAL_MULTIPLIER 由 17 改为 19（2026-09-25）

- 指令：管理员在对话中要求「把当前的隐藏倍率提高到19」。
- 通道：本机诊断密钥 SSH 到生产 Mac。改 `.env` 并重建应用容器，这是本次唯一的写操作，没有 API 可用；其余核对都是只读。
- 生效时间：**2026-09-25 10:25:24（+08）**，即 02:25:24Z、Mac 本地 11:25:24（JST）。
- 上一次改动（18 → 17，2026-09-17 14:42:20 +08）见 `20260917-155102-final-multiplier-18-to-17_CN.md`，本次流程与它相同。

## 改前检查（只读）

- `.env` 第 26 行 `BILLING_FINAL_MULTIPLIER=17`，全文件只有这一行；运行中的容器内也是 17。
- 运行中的镜像是 `sha-2a1f90bb…`（PR #58），`.env` 的 `IMAGE_TAG` 与之一致；容器 2026-09-24 06:52:10Z 启动。
- `deploy-mac.sh` 的倍率自检只在未设置时报警，不校验具体值，所以之后 CI 部署会沿用 19，不会被改回。
- 代码里只有 `billing_service.go` 的 `applyFinalBillingMultiplier` 读这个配置，只放大 `actual_cost`，不写库、不缓存，启动时读取，所以必须重建容器，且只影响重建之后的请求。
- **配置哈希比对**：`docker compose -f docker-compose.yml -f docker-compose.mac.yml config --hash sub2api` 与容器标签 `com.docker.compose.config-hash` 相同（`bccee9f2…`），说明重建只会带进这次改动。

## 执行

脚本一次跑完，任一检查不过就恢复备份并中止；健康检查不过会自动回滚到 17。

1. 备份：`.env.bak-20260925-112523-final-multiplier-17`（Mac 本地时间命名），权限 600。
2. 用 `/usr/bin/sed` 只替换 `^BILLING_FINAL_MULTIPLIER=17$` 这一行；与备份 diff，只有这一行不同。
3. 渲染比对：分别用备份和新 `.env` 渲染完整配置再 diff，唯一差异是 `BILLING_FINAL_MULTIPLIER: "17"` → `"19"`。四项检查：镜像不变、端口仍只绑 `127.0.0.1:8080`、倍率 `"19"`、`account_credentials_encryption_key` secret 仍在。新哈希 `d201dc26…`。
4. 重建：`docker compose … up -d --no-deps sub2api`。02:25:23Z 开始，02:25:24.17Z 新容器启动，第 2 次健康检查（02:25:26Z）通过，停服约 3 秒。postgres、redis 的启动时间仍是 09-08，没有被动到。
5. 验证：
   - 容器内 `BILLING_FINAL_MULTIPLIER=19`，标签哈希 = 新哈希，镜像仍为 `sha-2a1f90bb…`；
   - 公网 `https://aaccx.pw/health` 返回 200；
   - 新容器启动日志 74 行，没有 error、fatal 或 panic；
   - 02:20Z 起 `ops_error_logs` 只有 4 条 403「余额不足」，是同一个分组 10 的客户端每 2 分钟一次的请求，改前改后都有，与重建无关。

## 生效核对

`usage_logs` 从切换前 3 小时起，按行内快照倍率反推最终倍率，即 `actual_cost ÷ (total_cost × rate_multiplier)`：

| 阶段 | 行数 | 反推最终倍率 | 时间范围（+08） |
| --- | --- | --- | --- |
| 切换前 | 46 | 全部 17 | 09:22:10 ～ 09:40:56 |
| 切换后 | 4 | 全部 19 | 10:36:36 ～ 10:38:13 |

- 切换后的第一条请求出现在 10:36:36，距切换约 11 分钟，这个时段流量很低。
- 切换后的 4 条都是分组倍率 0.35 的 `gpt-5.6-terra`，逐条满足 `actual_cost = total_cost × 0.35 × 19`，精确相等。例如 id 438677：0.034292 × 6.65 = 0.2280418。
- 切换后没有 `total_cost = 0` 的行。

## 影响

- 所有分组的实际扣费变为原来的 19/17，即**上涨约 11.8%**。广场展示不变，因为广场本来就不含隐藏倍率。
- **`usage_logs` 不记录最终倍率**。核对历史扣费时按 `created_at` 分段（+08）：09-17 14:42:20 之前乘 18，到 09-25 10:25:24 之前乘 17，之后乘 19。
- **换算口径变化**（最便宜的 ¥29 套餐、额度用满，1 元 = 10.48 余额美元；广场口径按 $10.45）：
  - 广场每 $1 实际花费：约 1.63 元 → 约 1.82 元；
  - 国产分组实付：「官网价 × 分组倍率 × 0.24」→「× 0.27」；2.8 倍率由约 6.7 折变为约 7.5 折，7.0 倍率由约官网价的 1.67 倍变为 1.87 倍；
  - 上游倍率与我方分组倍率相同的分组，收入/成本比就是 `19 ÷ 10.48 ≈ 1.81`（原 1.62），例如 GLM 76（4.2 对 4.2）；
  - Claude 84（0.75 对 0.75）：约 1.82～2.00 倍，保本分组倍率约 0.38～0.41（原 1.63～1.79 倍、0.42～0.46）；
  - GPT 80（1.0 对 1.0）：约 1.82 倍，保本线 0.55（原 1.63 倍、0.615）；
  - Kimi 7（7.0 对 4.9）：约 2.59 倍（原 2.32 倍）；
  - 81～83（2.8 对 4）：约 1.27 倍，保本分组倍率约 2.21（原 1.14 倍、2.47）；
  - 已停用的火神 77～79：约 14.0 倍（原 12.5 倍）。
  - 管理员赠送、兑换码、8% 邀请返利带来的余额没有收入，会拉低这些比例。
- `batch_image.enabled` 若被打开，会少收 18/19 ≈ 94.7%（仍保持关闭）。
- 仓库里的 `deploy/docker-compose.18082.yml` 写死了 18，它是本地测试实例、不是生产配置，本次没有改。

## 回滚

```bash
cd ~/sub2api
/usr/bin/sed -i "" -E 's/^BILLING_FINAL_MULTIPLIER=19$/BILLING_FINAL_MULTIPLIER=17/' .env
~/.orbstack/bin/docker compose -f docker-compose.yml -f docker-compose.mac.yml config | grep BILLING_FINAL_MULTIPLIER
~/.orbstack/bin/docker compose -f docker-compose.yml -f docker-compose.mac.yml up -d --no-deps sub2api
```

备份的 `.env` 里 `IMAGE_TAG` 是 `sha-2a1f90bb…`。之后如果 CI 部署过新版本，就不能整份复制备份文件，只能像上面这样用 sed 改倍率这一行，否则会把镜像一起回滚。
