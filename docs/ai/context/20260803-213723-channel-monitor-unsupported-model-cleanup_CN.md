# 渠道监控不支持模型清理

## 原因

渠道监控器使用通用文本请求。图像模型和特殊多代理模型不接受该协议，会稳定产生错误记录，干扰模型监控明细。

## 删除项

- GPT：`gpt-image-1.5`、`gpt-image-2`。
- Grok：`grok-4.20-multi-agent-0309`、`grok-imagine-image`、`grok-imagine-image-quality`。
- Claude Max：`claude-haiku-4-5-20251001`。

## 保留项

- GPT：`gpt-5.5`、`codex-auto-review`、`gpt-5.6-luna`、`gpt-5.6-sol`、`gpt-5.6-terra`。
- Grok：`grok-4.3`、`grok-4.20-0309-non-reasoning`、`grok-4.20-0309-reasoning`、`grok-4.5`、`grok-build-0.1`。
- Claude Max：保留 Opus、Sonnet 与 Fable 文本模型。

## 验证

已在单个事务中更新三条渠道监控，并读取 `channel_monitors.extra_models` 确认删除项不再存在。历史检测记录未删除，保留用于审计；用户页面会按当前模型配置展示。
