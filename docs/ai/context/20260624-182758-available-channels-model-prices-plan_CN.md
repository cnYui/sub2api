# Available Channels 页面收敛为模型价格页计划

## 目标

- 让 `/available-channels` 只保留价格卡展示。
- 将页面标题/描述改为“模型价格”语义。
- 用最小改动完成前端视图收敛，并保留当前价格数据链路。

## 涉及文件

- 修改：`frontend/src/views/user/AvailableChannelsView.vue`
- 修改：`frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
- 修改：`frontend/src/i18n/locales/zh.ts`
- 修改：`frontend/src/i18n/locales/en.ts`

## 执行步骤

1. 先改测试：
   - 更新 `AvailableChannelsView.spec.ts`
   - 保留价格卡断言
   - 新增/调整断言，确认页面不再渲染 `available-channels-table`
2. 运行目标测试，确认在现状下失败。
3. 修改 `AvailableChannelsView.vue`：
   - 移除搜索框、刷新按钮、表格插槽
   - 删除仅供表格使用的状态、计算属性和组件引用
   - 保留 `loadChannels()` 与价格卡渲染逻辑
4. 修改中英文 i18n 文案：
   - `availableChannels.title`
   - `availableChannels.description`
5. 再次运行目标测试，确认通过。
6. 运行 `frontend` build，确认类型检查和打包通过。
7. 写结果文档到 `docs/ai/context/`。

## 验证命令

```bash
pnpm -C frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts
pnpm -C frontend build
```

## 预期结果

- 页面顶部仅保留价格卡。
- 路由标题显示“模型价格”。
- 不再出现渠道表和搜索控件。
- 目标测试与前端 build 通过。
