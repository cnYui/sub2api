# Claude Desktop 接入教程更新

## 变更目的

将登录后 `/usage-guide` 中原“Claude Code 桌面端接入”教程替换为用户指定的 Claude Desktop 接入中转站 Claude 渠道模型方法。

## 实现内容

- 主题 ID 改为 `claude-desktop`，标题改为“Claude Desktop 接入中转站 Claude 渠道模型方法”，更新时间为 `2026-08-11`。
- 教程重写为 4 步：点击 Cloud Desktop 和加号；创建密钥并填写 API Key 与 `https://api.aaccx.pw`；添加、启用并重启 Claude Desktop；选择模型开始聊天。
- 第 2 步展示创建密钥、填写字段和完成结果 3 张图；第 4 步展示退出菜单和模型列表 2 张图。
- 用户提供的 7 张截图复制到 `frontend/src/assets/usage-guide/claude-desktop-step-*.png`，旧 Claude Code 教程专用的 6 张截图已删除。
- 更新 `UsageGuideView` 定向测试，改为检查新主题、日期和 7 张资源。

## 约定

- 请求地址按截图和用户要求使用 `https://api.aaccx.pw`，不追加 `/v1` 或尾部斜杠。
- API Key 继续只在截图示例中使用脱敏或掩码值，页面不写入真实凭证。

## 验证

- 运行 `frontend` 使用方法页面单测，确认主题源码和全部截图资源存在。
- 运行前端类型检查和生产构建，确认新图片可被 Vite 静态导入。
