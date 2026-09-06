# 最终倍率 18 是否仍生效——三重独立核验

- 时间：2026-09-05 20:35（+09）
- 触发：管理员在 GHCR 首次部署（`20260905-200812-*`）之后复查隐藏倍率
- 结论：**仍然生效**，近 7 天 27,381 条计费记录零例外
- 性质：核验记录 + 三个新发现的坑

> 本仓库是公开仓库。运维敏感值一律用 `${变量名}` 占位。

---

## 1. 为什么不能只看环境变量

`docker exec sub2api env | grep BILLING` 返回 `BILLING_FINAL_MULTIPLIER=18` —— 这条证据
**不足以下结论**，项目里有三条现成的反例路径：

- 坑 3：基础 compose 没有 `env_file:`，`.env` 的变量不会自动进容器；
- 坑 4：JWT secret 的**实际生效值来自 `/app/data/config.yaml`** 而不是 `.env`，
  即「环境变量存在」不等于「应用读的是它」；
- 坑 12：取不到定价时静默按 0 计费放行，配置对了扣费仍可能是错的。

所以按「运行态 / 数据库实证 / 源码链路」三条独立线交叉验证。

---

## 2. 三重证据

### ① 数据库实证（最硬，主证据）

用 **numeric 精确等值比较**（不是浮点容差），除数取**行内快照倍率**：

```sql
SELECT count(*) AS rows_costed,
       count(*) FILTER (WHERE actual_cost = total_cost * rate_multiplier * 18) AS eq_18,
       count(*) FILTER (WHERE actual_cost <> total_cost * rate_multiplier * 18) AS not_18
FROM usage_logs
WHERE created_at >= now() - interval '7 days' AND total_cost > 0;
```

| 窗口 | 计费记录 | 等于 18 | 不等于 18 |
| --- | --- | --- | --- |
| 近 7 天 | 27,381 | **27,381** | **0** |
| 容器重建（11:06:58Z）之后 | 21 | **21** | **0** |

反推倍率的极值：`min = max = 18.00000000`，**distinct 值只有 1 个**。

零成本行单独核查：近 7 天 26 条 `total_cost = 0`，**token 数也全为 0**
（`has_tokens_but_zero_cost = 0`）——零 token 得零成本算术上正确，
**不是**坑 12 的少收场景，与既有的「零 token 占位记录」一致。

### ② 运行态配置

- 从**宿主机**读活进程 `/proc/<容器PID>/environ`（容器内 root 因 `CapEff` 全零、
  无 `CAP_SYS_PTRACE` 读不到 `/proc/1/environ`）：`BILLING_FINAL_MULTIPLIER=18`
  **唯一一条**，无重复键遮蔽。`RestartCount=0`，进程启动时间与容器 `StartedAt` 一致。
- 覆盖路径全部排除：运行态 `config.yaml` **没有 `billing:` 段**
  （其 `default.rate_multiplier: 1` 是新建分组的默认倍率，与最终倍率无关）；
  `settings` 表 414 行无任何 `final_multiplier` / `billing.*` 键。
- 变量来源：`.env` → compose `${}` 插值 → `docker-compose.vps.yml` 的 `environment:` 显式列项。
  基础 compose 的 61 个键里 `BILLING*` 为 0，override 再加 5 个，加上镜像自带的 `PATH`，
  合计 `61 + 5 + 1 = 67` 与 `Config.Env` 一致。

### ③ 源码链路

- 全仓库只有 **5 个最终倍率乘入点，全部在 `billing_service.go`**，
  覆盖 token / 按次 / WebSearch / 图片 / 视频五种计费模式。
- 在线网关全部收口：`applyUsageBilling` 的非测试调用点只有
  `gateway_usage_billing.go:780` 与 `openai_gateway_usage.go:387`。
- 余额扣费、订阅扣费、流量卡扣费、欠费账本写入四条全部使用**已乘倍率**的 `ActualCost`；
  账号配额刻意用未乘的 `TotalCost × 账号倍率`，符合「账号统计倍率不参与用户扣费」的约定。
- 生产镜像 `sha-497cbda67` 之后 `backend/` 零改动（本地 HEAD 领先的提交全是文档）。

---

## 3. 关于 11:06 那次重建

`.env` 与 `docker-compose.vps.yml` 的 mtime 都是 11:06，容器随即重建——正是历史上
最容易丢掉 18 倍的场景。核验结果：**没有漂移**。

- 逐行值哈希对比（`.env` 含口令，不直接 diff，改比 `KEY=sha256(值)前12位+len`）：
  相对 `.bak.20260905-110618` 只多出末行 `IMAGE_TAG`，前 32 行逐行一致，
  `BILLING_FINAL_MULTIPLIER` 的值哈希未变且确认就是 `18`。
- compose 只改镜像行，字节数 `1989→2016 = +27`，正好等于该行长度差，无隐藏改动空间。
- 容器 `com.docker.compose.project.config_files` 标签同时列出两个 compose 文件，坑 1 未触发。
- 端口仍只绑 `127.0.0.1:8080`；`DOCKER-USER` 链的 `ESTABLISHED,RELATED RETURN`
  仍排在 `DROP` 之前（坑 2 未触发）。

**但要准确表述**：配置变量一个字节没动，**计费代码整体换了一版**。
`497cbda6` 包含 `7bf476698`，本次同时上线了 `billing_service.go`(+36)、
`pricing_service.go`(+33)、`openai_model_alias.go`(+25)、`payment_refund.go`(+26)。

---

## 4. 三个新发现

### ① 假异常：不能 JOIN `groups.rate_multiplier` 核对历史扣费

核验中一度报出「DeepSeek 分组比值恒为 `12.857143`」。实际是 `18 × 3.5 ÷ 4.9`——
该分组当天刚从 `3.5x` 改成 `4.9x`，那 208 条是改之前扣的，行内快照倍率是 `3.5`。
按快照倍率重算全部精确 18；反向验证 `actual_cost = total_cost × 4.9 × 18` 成立的行数为 **0**。

**已写入 CLAUDE.md 坑 25。** 这类假异常看起来极有依据，与坑 16 同类。

### ② 默认值是 1.0，但 vps override 有 `:?` 硬门禁

`config.go` 的 `viper.SetDefault("billing.final_multiplier", 1.0)`，且配置校验只拒绝
NaN/Inf/≤0——**1.0 是合法值**，变量丢失时容器正常启动、健康检查通过、日志一字不报，
只能靠账单反推。

缓解：`docker-compose.vps.yml` 用的是 `${BILLING_FINAL_MULTIPLIER:?...}` **必填语法**，
带 `-f` 就会直接启动失败而非静默上线；`watchdog.sh`、`finish-deploy.sh`、
`restore-cutover.sh` 也都带全了两个 `-f`。

**残留风险只在人工裸敲 `docker compose up -d` 漏掉 override 时。**
可选加固（未执行）：`.env` 增加
`COMPOSE_FILE=docker-compose.yml:docker-compose.vps.yml`，让 override 自动叠加。

顺带：`${IMAGE_TAG:-main}` 是软失败——`IMAGE_TAG` 丢失会静默拉移动标签 `main`
（另一份计费代码），与紧邻的 `:?` 强校验风格不一致。

### ③ 坑 12 的排查串在两条网关上不一样

`pricing_missing_record_zero_cost` **只在 OpenAI 链路打**。
通用网关（Claude/Gemini/Grok/GLM/DeepSeek/Kimi）在
`gateway_usage_billing.go:919/970` 吞错返回 `ActualCost: 0` 时打的是
`"Calculate image token cost failed"` / `"Calculate cost failed"`。

**只 grep 前者查全链路必然漏。** 本次两串都查过，均为 0 条。

---

## 5. 未纳入本次范围

- `gpt-5.6-sol` 超收（管理员已决定保持现状）。
- 长上下文计费仍未开启：`gpt-6-astra` 的静态价随本次部署上线了，
  但账号级开关 `openai_long_context_billing_enabled` 默认 false。
  **CLAUDE.md 五点五「镜像早于 `7bf476698`、生产完全没登记」的表述已过期。**
- Live(WebRTC) 路径零计费缺陷，另行处理。
- 各分组 `rate_multiplier` 取值是否符合业务预期。
- 17 个分组中 8 个近 7 天零流量（Gemini 70、Image-2生图 12、GLM 6、Claude 4/5/72/73、
  已停用 Grok 3），属**未观测**而非已验证，其中含走不同网关代码的链路。

---

## 6. 一个安全事项

核验期间某个子代理为确认 `config.yaml` 无 `billing:` 段而 dump 了文件行区间，
**JWT secret 出现在了会话工具输出中**。未写入任何结论文本或文件，但本地 transcript
已记录。按坑 4，JWT secret 的实际生效值就来自该文件，建议轮换一次
（代价：全部用户网页会话失效）。
