# 新手引导弹窗合并验证补充

## 本次改动验证

- 引导定向测试：3 项通过。
- TypeScript 类型检查：通过。
- 前端生产构建：通过。
- 本地应用容器健康检查与登录页：通过。

## 全量前端测试现状

`pnpm test:run` 共执行 895 项断言，其中 885 项通过、10 项失败。失败均位于本次未修改的模块：

- `src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `src/components/charts/__tests__/ModelDistributionChart.spec.ts`
- `src/components/charts/__tests__/GroupDistributionChart.spec.ts`
- `src/composables/__tests__/usePersistedPageSize.spec.ts`

失败内容分别涉及历史图片用量英文文案、图表费用字段与灰阶颜色预期，以及分页默认值，和引导弹窗的 CSS、步骤文案规范化逻辑不存在调用关系。本次按用户要求仅提交引导弹窗相关文件，不处理或掩盖上述既有失败。
