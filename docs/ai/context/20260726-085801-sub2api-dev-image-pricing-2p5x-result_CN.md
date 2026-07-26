# 18080 图片生成模型计费调整到 2.5x 结果

时间：2026-07-26 08:58:01

## 已完成的运行态配置修改

已更新外层 `sub2api-dev` 的挂载价格文件：

- `deploy/data/model_pricing.json`
- `deploy/data/model_pricing.sha256`

处理范围仅为 `mode: image_generation` 的 12 个模型：6 个 Gemini 图片模型与 6 个 GPT 图片模型。每个可计费字段（含图片、文本、音频、缓存、Batch 和图片模型内的检索费用）均从原值乘以 `1.25`。

全局 `billing.unit_price_multiplier` 保持 `2.0`，因此图片模型最终基础计费从 `2.0x` 变为 `2.5x`。非图片模型与配置文件其他条目经 JSON 结构比对均未改变。

价格文件 SHA-256 已更新为：

```text
a6ea78219e851f4829f22a83fadb7f89f2dcc2cb0b30e32f6f8e2824539be670
```

## 备份

- `deploy/backups/20260726-084508-sub2api-dev-model-pricing-pre-image-2p5x.json`
- `deploy/backups/20260726-084508-sub2api-dev-model-pricing-pre-image-2p5x.sha256`

## 未能完成的生效验证

发现 `sub2api-dev` 在本次操作前已进入重启循环；未由本次价格文件修改触发。容器启动失败原因为既有迁移错误：

```text
apply migration 178_seed_codex_249_299_subscription_plans.sql:
pq: column "image_rate_independent" of relation "groups" does not exist
```

截至记录时：

- `sub2api-dev`：`restarting`，退出码 `1`，重启次数 `11`
- `http://127.0.0.1:18080/health`：连接被拒绝

因此价格文件已写入并完成静态校验，但尚未被 18080 的进程加载。修复数据库迁移链属于独立的运行态/Schema 变更，本次未擅自执行。

## 回滚

将上述两份备份恢复到 `deploy/data/` 后，在服务恢复启动条件后重启 `sub2api-dev`。
