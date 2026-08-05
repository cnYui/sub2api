# 生图方法主题下架

## 需求

从 `main` 分支的使用方法页面下架并隐藏“生图方法”主题。

## 实现决策

- 仅从使用方法页面的可见主题列表中隐藏 `image-generation`，不删除原有主题数据、接口说明或请求示例，便于后续恢复和审计。
- 使用 `allGuideTopics` 保存完整主题，使用 `hiddenGuideTopicIds` 过滤桌面侧栏和移动端标签；两种导航继续复用同一组件数据源。
- 本次不改变生图 API、权限、计费或其它页面行为。

## 变更范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

## 验证结果

- 使用方法页面单测通过，包含主题隐藏规则和资源存在性检查。
- 类型检查、Lint 和生产构建通过。
