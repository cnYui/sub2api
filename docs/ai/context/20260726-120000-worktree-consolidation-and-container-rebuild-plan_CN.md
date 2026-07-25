# 本地工作区整理与容器重建计划

## 目标

整理本地 `main` 的未提交和未跟踪内容，合并已完成的周额度流量卡兜底分支，然后重新构建并启动本地 `18080` 与内层 `18086` Sub2API 容器。

## 已盘点内容

- 已暂存：OpenAI 自动透传请求前计费预授权修复及其测试、相关排查与结果文档。
- 未暂存：`billing.unit_price_multiplier` 的配置、基础定价缩放逻辑及测试。
- 未跟踪：`pricing_multiplier.go` 和 71 份历史上下文文档。
- 未合并分支：`codex/rolling-weekly-traffic-credit-fallback`，仅在滚动周额度不足时进入既有流量卡 reservation 路径，并带有测试与结果文档。

## 执行顺序

1. 扫描所有待提交内容，确认没有完整凭据或密钥。
2. 将预授权修复、基础定价倍率、历史上下文文档按功能提交，并跟踪全部未跟踪文件。
3. 合并 `codex/rolling-weekly-traffic-credit-fallback`，执行全量后端测试。
4. 按容器标签确认的 Compose 项目分别重建 `sub2api-dev:18080` 与 `sub2api-upstream-latest:18086`；不重建 PostgreSQL、Redis、Nginx 或其他项目容器。
5. 检查两个健康检查端点和容器状态，记录结果与回滚边界。
