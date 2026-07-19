# 删除 using-superpowers 技能计划

## 背景

用户要求查找当前电脑里使用的 `using-superpower` 技能所有位置并删除该技能。实际技能名为 `using-superpowers`。

## 已定位的技能目录

- `/Users/wujianxiang/.codex/skills/using-superpowers`
- `/Users/wujianxiang/.codex/plugins/cache/openai-api-curated/superpowers/11c74d6b/skills/using-superpowers`
- `/Users/wujianxiang/.codex/plugins/yui-global-rules/skills/using-superpowers`
- `/Users/wujianxiang/.codex/.tmp/plugins/plugins/superpowers/skills/using-superpowers`

## 删除边界

- 只删除上述 4 个精确技能目录。
- 不删除 `systematic-debugging`、`test-driven-development` 等其他 superpowers 技能。
- 不清理历史会话日志、全局状态或 README 中的纯文本引用，避免破坏审计记录或误删包含敏感历史内容的文件。
- 插件缓存目录删除后，如果后续插件重新安装或刷新，可能再次生成同名技能，需要届时再移除或从插件源头禁用。

## 验证

- 删除后运行 `find /Users/wujianxiang/.codex -path '*/skills/using-superpowers' -type d -print`，确认无技能目录残留。
