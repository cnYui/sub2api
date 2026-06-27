# Priority/Fast 1.5 倍计费修改结果

## 结果

已把 OpenAI `service_tier=priority` 以及客户端别名 `fast` 的计费规则从 2 倍改为 1.5 倍。

核心变化：

- `fast` 仍在入口归一化为 `priority`。
- `BillingService` 不再直接使用 LiteLLM / fallback 价格表里的 2 倍 priority 单价进行扣费。
- `priority` 统一作为服务等级倍率处理，倍率为 `1.5`，覆盖输入、输出、缓存写入、缓存读取和图片输出 token 成本。
- 模型价格输出字段仍保留 priority 展示价，但统一按基础价 `* 1.5` 计算，保证 `/api/v1/channels/prices` 和真实扣费口径一致。
- 渠道覆盖价也会生成对应的 1.5 倍 priority 展示价，避免价格来源不同导致规则分叉。

## 当前展示价

- `gpt-5.4` 基础价：输入 `$2.5/1M`，输出 `$15/1M`，缓存读取 `$0.25/1M`。
- `gpt-5.4` priority/fast：输入 `$3.75/1M`，输出 `$22.5/1M`，缓存读取 `$0.375/1M`。
- `gpt-5.5` 基础价：输入 `$5/1M`，输出 `$30/1M`，缓存读取 `$0.5/1M`。
- `gpt-5.5` priority/fast：输入 `$7.5/1M`，输出 `$45/1M`，缓存读取 `$0.75/1M`。

## 验证

- `go test -tags unit ./internal/service ./internal/handler ./internal/server/routes ./cmd/server`
- `pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts`
- `pnpm --dir frontend build`

以上均通过。前端 build 仍有项目既有的 Vite chunk / Browserslist warning，不影响本次结果。

## 运行态说明

本次修改已落到源码和前端构建产物；公网正式容器/本地运行进程需要按正常发布或重启流程加载新代码后，真实请求才会按 1.5 倍扣费。`docs/ai/context/20260621-202151-openai-fast-priority-billing-test-result_CN.md` 中的 2 倍结果是修改前历史验证记录。
