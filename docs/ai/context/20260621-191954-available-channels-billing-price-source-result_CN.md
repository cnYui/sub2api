# `/available-channels` 实际计费价格展示结果

## 完成内容

- 修复 `/available-channels` 顶部价格摘要只显示生图、不显示 GPT 价格的问题。
- 根因：上一版前端从 `/api/v1/channels/available` 的渠道列表中派生 GPT 价格；当前运行态渠道表为空时，价格摘要也拿不到 `gpt-5.4/gpt-5.5`。
- 新增用户侧只读接口 `GET /api/v1/channels/prices`：
  - 复用后端 `ModelPricingResolver` 的实际计费口径。
  - 返回 `gpt-5.4`、`gpt-5.5` 的 token 单价、缓存读取价、priority 单价和价格来源。
  - 未登录返回 401，登录后按 `available_channels_enabled` 开关返回。
- 前端 `/available-channels`：
  - 顶部「当前价格」优先读取 `/channels/prices`。
  - 渠道表为空时仍展示 `gpt-5.4`、`gpt-5.5` 和生图价格。
  - 新价格接口异常时，回退到原来的渠道模型定价。
  - 副标题改为「按实际计费配置与当前账号可用分组计算」。
- `/monitor` 普通用户侧边栏入口继续保持隐藏；管理员监控入口不变。

## 当前展示价格口径

- `gpt-5.4`：
  - 输入：`$2.5 / 1M token`
  - 输出：`$15 / 1M token`
  - 缓存写入：`$0 / 1M token`
  - 缓存读取：`$0.25 / 1M token`
  - priority 输入：`$5 / 1M token`
  - priority 输出：`$30 / 1M token`
  - priority 缓存读取：`$0.5 / 1M token`
- `gpt-5.5`：
  - 输入：`$5 / 1M token`
  - 输出：`$30 / 1M token`
  - 缓存写入：`$0 / 1M token`
  - 缓存读取：`$0.5 / 1M token`
  - priority 输入：`$10 / 1M token`
  - priority 输出：`$60 / 1M token`
  - priority 缓存读取：`$1 / 1M token`
- 生图：
  - 1K：`$0.10 / 张`
  - 2K：`$0.20 / 张`
  - 4K：`$0.40 / 张`

## 验证

```bash
go test -tags unit ./internal/handler -run 'TestToUserModelPrice_FromResolvedPricing|TestUserAvailableChannel'
go test -tags unit ./internal/handler ./internal/server/routes ./cmd/server
pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts
pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts
pnpm --dir frontend build
```

全部通过。

## 本地预览

- 预览后端：`http://127.0.0.1:18081`
- 预览前端：`http://localhost:5174/available-channels`
- `http://localhost:5174/api/v1/channels/prices` 未登录返回 401，确认新路由已挂载；登录后由页面请求并展示价格。
- 未改动正式公网入口 `127.0.0.1:18080` 对应容器。
