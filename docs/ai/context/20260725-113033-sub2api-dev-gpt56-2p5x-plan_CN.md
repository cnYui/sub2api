# 18080 gpt5.6 三款模型计费从 2x 调整到 2.5x 计划

时间：2026-07-25 11:30:33

## 目标

只将当前 18080 外层 `sub2api-dev` 中三款 gpt5.6 模型调整为 `2.5x` 计费：

- `gpt-5.6-luna`
- `gpt-5.6-sol`
- `gpt-5.6-terra`

其他模型继续保持现有全局 `billing.unit_price_multiplier: 2.0`。

## 当前事实

- 18080 对应容器：`sub2api-dev`
- 运行态配置：`deploy/data/config.yaml` 挂载到容器 `/app/data/config.yaml`
- 当前全局倍率：`billing.unit_price_multiplier: 2.0`
- 当前运行态价格文件：`deploy/data/model_pricing.json`
- 默认价格远程哈希同步每 10 分钟检查一次；如果只改本地价格文件，不关闭远程同步，可能被远程价格覆盖。

## 方案

- 不修改全局 `billing.unit_price_multiplier`，避免所有模型涨价。
- 将 `deploy/data/model_pricing.json` 中三款 gpt5.6 的可计费单价字段整体乘 `1.25`。
- 保持全局 2x 后，三款模型的最终用户侧基础计费变为 `2.0 * 1.25 = 2.5x`。
- 在 `deploy/data/config.yaml` 显式加入 `pricing.hash_url: ""` 与较长 `pricing.update_interval_hours`，避免运行态本地定制价格被默认远程哈希同步覆盖。
- 修改前备份运行态配置与价格文件到 `C:\tmp\sub2api-config-backups\`。

## 验证

- 重启 `sub2api-dev`。
- 验证 `127.0.0.1:18080/health` 返回 200。
- 验证容器日志重新加载 `./data/model_pricing.json`。
- 核对三款模型运行态价格文件：
  - `luna` 标准 input 从 `1e-06` 改为 `1.25e-06`，最终 `2.5 USD/MTok`。
  - `terra` 标准 input 从 `2.5e-06` 改为 `3.125e-06`，最终 `6.25 USD/MTok`。
  - `sol` 标准 input 从 `5e-06` 改为 `6.25e-06`，最终 `12.5 USD/MTok`。

## 回滚

- 恢复备份的 `config.yaml`、`model_pricing.json`、`model_pricing.sha256`。
- 重启 `sub2api-dev`。
- 历史用量不回算，只影响新请求。
