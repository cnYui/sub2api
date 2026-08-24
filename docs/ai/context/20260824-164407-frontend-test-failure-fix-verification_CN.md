# 前端全量测试失败修复与验证

## 问题含义

- 系统回滚 API 断言缺少请求超时配置，实际实现已传入 15 分钟超时，测试仍按旧调用签名校验。
- 首页紧凑模式断言使用旧站点名 `Test site`，当前固定品牌文案已为 `Genius Programmer Hub`。
- 支付方式选择器断言使用旧边框 class，当前选中样式为 `border-primary-500/70`。
- 两个 GroupsView 测试未 mock 新增的 `getLiveCapability` API，组件挂载时触发真实未处理调用，产生 unhandled error。

## 修复

- 更新回滚 API 两个调用断言，包含 `{ timeout: 15 * 60 * 1000 }`。
- 将首页品牌文案断言改为当前固定文案。
- 将支付方式选中边框断言改为当前 class。
- 为 `GroupsView.columnSettings.spec.ts` 和 `GroupsView.duplicate.spec.ts` 补充 `getLiveCapability` mock，并在每个测试前重置为 `{ supported: false }`。

## 验证

- `pnpm typecheck` 通过。
- 前端定向修复测试：5 个文件、24 个测试通过。
- 前端全量 Vitest：200 个测试文件通过，1392 个测试通过，0 failed，0 unhandled errors。
- 测试输出中的 Vue router-link、故意触发的网络错误和 Browserslist 提示均为既有测试日志/环境提示，不影响退出状态。

本次仅修改测试断言和 mock，不提交、推送、重启 Docker 或部署公网。
