# 删除 using-superpowers 技能结果

## 操作结果

已删除以下 4 个 `using-superpowers` 技能目录：

- `/Users/wujianxiang/.codex/skills/using-superpowers`
- `/Users/wujianxiang/.codex/plugins/cache/openai-api-curated/superpowers/11c74d6b/skills/using-superpowers`
- `/Users/wujianxiang/.codex/plugins/yui-global-rules/skills/using-superpowers`
- `/Users/wujianxiang/.codex/.tmp/plugins/plugins/superpowers/skills/using-superpowers`

## 验证

执行以下命令后均无输出，表示 `~/.codex` 下没有残留的同名技能路径：

```bash
find /Users/wujianxiang/.codex -path '*/skills/using-superpowers' -type d -print | sort
find /Users/wujianxiang/.codex -iname '*using-superpower*' -print | sort
```

## 注意

- 当前会话的技能列表由会话启动时注入，不会因为文件删除立刻从本会话上下文中消失；新会话或刷新后才会体现。
- 插件缓存中的副本已删除，但后续如果插件重新安装或刷新，可能重新生成该技能。
- 历史会话日志和 README 中的纯文本引用未删除，避免破坏审计记录或误删包含敏感历史内容的文件。
