# Codex CC Switch KIMI 教程验证

## 已完成验证

- `pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts`：通过，3 个测试全部成功。
- `pnpm typecheck`：通过。
- `pnpm build`：通过，生产构建已打包 `codex-ccswitch-step-01.png` 至 `codex-ccswitch-step-10.png`。
- `git diff --check`：通过。
- 对 10 张新增图片检查后，原始完整 API Key 字符串未保留；第 6、9 张图片均显示 `sk-xxxx`。

## 已知非阻断项

- 构建保留项目既有的 Browserslist 数据过期、动态导入分包和大 chunk 警告，未修改非本次范围的构建配置。
- 尝试启动本地 Vite 服务以人工访问 `/usage-guide` 时，后台进程启动被当前执行策略拦截；未影响测试、类型检查和生产构建结果，也未触及公网服务。
