# DeepSeek 分组展示名称修正

## 变更

将生产 `18082` 的公开 OpenAI 分组 `groups.id=8` 名称从：

`DeepSeek模型官方0.42折价格`

修正为：

`DeepSeek模型官方0.5折价格`

## 不变项

- 仅修改展示名称。
- 分组平台仍为 `openai`，状态仍为 `active`。
- `rate_multiplier` 保持 `3.5`，未变更用户计费或上游价格校准逻辑。

## 验证

认证用户接口 `GET /api/v1/groups/available` 已返回分组 ID `8` 的新名称及原倍率 `3.5`。

未重启、重建或替换公网 `18082` 容器；健康检查返回 200。
