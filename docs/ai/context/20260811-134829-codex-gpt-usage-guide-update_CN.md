# Codex GPT 模型接入教程更新

## 变更目的

在登录后 `/usage-guide` 新增独立的“Codex 接入 GPT 模型”教程，保留原有 Codex 接入中转站其它模型的教程不变。

## 实现内容

- 新增主题 ID `codex-gpt`，更新时间为 `2026-08-11`。
- 教程为 3 步：选择 GPT 分组并点击橙色加号；填写供应商名称、API Key 和请求地址并保存；启用供应商后重启 Codex。
- 请求地址按用户要求固定使用 `https://api.aaccx.pw`，不追加 `/v1` 或尾部斜杠。
- 用户提供的 4 张截图复制到 `frontend/src/assets/usage-guide/codex-gpt-step-*.png`。
- 第 3 张截图中的 `auth.json` 完整 API Key 已用 `sk-xxxx` 替换后再入库，原始截图未复制到项目。
- 更新 `UsageGuideView` 定向测试，检查新主题、日期和 4 张资源。

## 合并范围

- 上一轮 Claude Desktop 教程改动已在 `main` 提交 `43d00e8a3` 中。
- 本轮只提交 GPT 教程代码、测试、4 张脱敏截图、本上下文记录及 `AGENTS.md` 对应记录；工作区其它诊断、并发和部署文档保持未提交。

## 验证

- 运行使用方法页单测、前端类型检查、定向 ESLint 和生产构建。
- 使用 `git diff --check` 检查补丁格式，确认构建产物包含 4 张 GPT 教程截图。
