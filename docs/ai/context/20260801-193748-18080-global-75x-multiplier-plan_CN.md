# 18080 全局倍率调整至 7.5 倍计划

## 目标

按用户明确授权，将公网外层 `sub2api-dev:18080` 的 `billing.unit_price_multiplier` 从 `5` 调整为 `7.5`。

`gpt-5.5` 与 `gpt-5.6-sol/terra/luna` 的基础价格已在上一轮恢复为 OpenAI 官方 Standard 价格，本次保持不变。

## 已核验事实

- 外层容器状态为 `running/healthy`，`18080/health` 返回 HTTP 200。
- 当前配置目录 `deploy/data` 挂载到容器 `/app/data`，修改后仅需重启 `sub2api-dev` 加载配置。
- 当前 `billing.unit_price_multiplier` 为 `5`，无环境变量覆盖。
- 四个目标模型的短上下文基础价仍为官方标准价格。

## 执行与验证

1. 备份当前 `deploy/data/config.yaml`，验证 SHA-256 一致且内容可读。
2. 仅将全局倍率改为 `7.5`，不修改模型价格、数据库、Redis、套餐或额度。
3. 仅重启 `sub2api-dev`，不重启 Nginx、内层 `18086`、PostgreSQL 或 Redis。
4. 核验容器内实际配置、环境变量覆盖、`18080/18086/8080` 健康检查。

## 回滚边界

恢复本次备份中的 `config.yaml` 并仅重启 `sub2api-dev` 即可回到 `5` 倍。历史 usage 和已创建的价格快照不重算。
