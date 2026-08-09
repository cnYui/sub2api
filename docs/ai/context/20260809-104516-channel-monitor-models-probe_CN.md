# 渠道监控统一改为模型目录探测

## 需求

`/monitor` 不再通过 Chat Completions 或 Responses 发起真实推理请求，避免周期性探测消耗上游 token。所有渠道监控统一使用 `GET /v1/models`，检测间隔统一为 30 分钟。

## 实现口径

- `channel_monitors.api_mode` 和监控模板的 `api_mode` 统一为 `models`，数据库约束不再接受 `chat_completions` / `responses`。
- checker 每个监控只请求一次 `/v1/models`，解析 OpenAI 兼容响应的 `data[].id`；主模型和附加模型分别记录目录存在或缺失。
- 目录请求只设置鉴权与自定义请求头，不发送 JSON body，不调用推理端点，因此不产生推理 token。
- HTTP 非 2xx、网络错误或目录响应无法解析记为 `error`；HTTP 2xx 但目标模型不在目录记为 `failed`；目标模型存在记为 `operational`。
- 管理端创建/编辑监控和模板只展示目录探测模式；历史记录在迁移时统一转换为 `models`。

## 生产变更

- 迁移 `207_channel_monitor_models_probe.sql` 将生产监控、模板和默认值统一为 `models`，并重建 api_mode 约束。
- 12 条现有监控的 `interval_seconds` 统一为 `1800`，`jitter_seconds` 保持原值。
- 公开设置 `channel_monitor_default_interval_seconds` 统一为 `1800`，新建监控默认每 30 分钟探测。

## 验证

- 后端服务与管理端类型检查通过。
- 需要在发布后核验 12 条监控均为 `api_mode=models`、`interval_seconds=1800`，并通过监控手动运行确认上游收到 `GET /v1/models` 而非 POST 推理请求。

