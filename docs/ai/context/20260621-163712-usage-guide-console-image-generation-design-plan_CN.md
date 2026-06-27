# 使用方法控制台与生图方法页面设计计划

## 背景

用户确认将 `/usage-guide` 从单一长页面升级为“使用方法控制台”：保留全局左侧导航中的一个「使用方法」入口，进入页面后通过页面内二级导航切换不同教程。当前需要保留原 Codex 接入教程，并新增“生图方法”教程。后续可继续新增视频生成、其他模型接入方法等栏目。

## 修改边界

- 只修改前端 `/usage-guide` 页面、测试和协作记忆。
- 不修改后端接口、分组配置、支付、订阅、兑换码、API Key、计费逻辑或公网 nginx 配置。
- 不把管理员内部执行细节原样展示给用户；生图教程只展示用户需要知道的可用性、接口、扣费规则和请求示例。
- 保持全局侧栏仍只有「使用方法」一个入口，不新增「生图方法」主导航项。

## 推荐设计

- `/usage-guide` 页面内新增教程控制台布局：
  - 桌面端：页面内容区左侧为二级导航，右侧为教程内容。
  - 移动端：二级导航显示为顶部横向标签，方便触摸切换。
- 教程栏目数据化：
  - `codex`：现有 8 步 10 图的 Codex 接入教程。
  - `image-generation`：新增生图方法教程。
- `codex` 教程继续使用步骤卡片和图片。
- `image-generation` 教程使用说明区块和代码示例，不新增真实按钮，不展示完整 API Key 或内部 token。

## 生图方法用户文案要点

- 29/39/59 元在售套餐已支持生图。
- 使用已有 API Key，请求地址保持 `https://api.aaccx.pw/v1`。
- 支持 OpenAI 兼容图片接口：
  - `POST /v1/images/generations`
  - 可在支持 `image_generation` 工具的客户端中使用图片生成能力。
- 计费按图片实际分辨率消耗订阅日额度：
  - 1K：`$0.10 / 张`
  - 2K：`$0.20 / 张`
  - 4K：`$0.40 / 张`
- 示例请求使用 `Authorization: Bearer sk-xxxx` 占位，不记录真实 Key。

## 实现计划

1. 先修改 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`：
   - 断言页面存在 `guideTopics` 或等价数据结构。
   - 断言有 `codex` 与 `image-generation` 两个 topic。
   - 断言页面有桌面二级导航和移动端标签导航。
   - 断言 Codex 教程仍保留 8 个步骤和 10 张截图。
   - 断言生图教程包含价格、接口、请求地址和安全占位示例。
2. 运行该测试确认红灯。
3. 重构 `frontend/src/views/user/UsageGuideView.vue`：
   - 从 `usageGuideSteps` 改为 `guideTopics`。
   - 增加 `activeTopicId` 状态和 `activeTopic` 计算属性。
   - 模板增加页面内二级导航。
   - Codex topic 渲染步骤图片。
   - 生图 topic 渲染说明卡片、价格表和代码示例。
4. 运行新增测试确认绿灯。
5. 运行相关回归测试、类型检查、构建和 `git diff --check`。
6. 新建结果文档并在 `AGENTS.md` 追加本次记录。

## 验证计划

- `pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
- `pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/router/__tests__/guards.spec.ts`
- `pnpm typecheck`
- `pnpm build`
- `git diff --check`

## Tradeoff

- 不拆多个路由：`/usage-guide/codex`、`/usage-guide/image-generation` 后续可以做，但当前同页切换更轻量，用户进入“使用方法”后能直接看到所有教程分类。
- 不继续无限追加单页长内容：二级导航把教程按主题分组，未来新增视频或模型接入不会污染 Codex 接入教程。
- 不展示后台分组 id 和运行态验证细节：这些信息对终端用户没有直接价值，且容易增加误解和维护成本。
