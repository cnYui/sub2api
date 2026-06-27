# 用户侧隐藏 monitor 与可用渠道价格摘要结果

## 改动

- 普通用户侧边栏已移除 `/monitor` 的「渠道状态」入口。
- `/monitor` 路由、`/api/v1/channel-monitors` 用户只读接口、管理员 `/admin/channels/monitor` 页面均未删除。
- `/available-channels` 搜索栏下方新增「当前价格」摘要：
  - `gpt-5.4`：从 `/channels/available` 返回的用户可见模型定价读取。
  - `gpt-5.5`：从 `/channels/available` 返回的用户可见模型定价读取。
  - 生图：从现有 `/groups/available` 返回的用户可见分组 `image_price_1k/2k/4k` 读取；如果分组未返回生图价格，则回退读取可见图片模型定价。
- 新增中英文 i18n 文案和前端测试。

## 验证

- 已按 TDD 先写失败测试：
  - 用户侧导航不应再声明 `path: '/monitor'`，但管理员 `/admin/channels/monitor` 保留。
  - `/available-channels` 应展示 `gpt-5.4`、`gpt-5.5`、生图和 1K/2K/4K 单价。
- 验证命令：

```bash
pnpm --dir frontend test:run src/components/layout/__tests__/AppSidebar.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts
pnpm --dir frontend build
```

结果：两条命令均通过。build 仅出现既有 Vite chunk/Browserslist warning，未产生构建产物 git 变更。

## 影响范围

- 仅前端展示变更。
- 未修改后端、支付、订阅、API Key、计费、路由守卫和公网配置。
