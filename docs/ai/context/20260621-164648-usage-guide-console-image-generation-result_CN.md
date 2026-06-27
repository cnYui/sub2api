# 使用方法控制台与生图方法页面结果

## 结果

已将 `/usage-guide` 从单一长页面改为页面内“使用方法控制台”：

- 全局左侧导航仍只有「使用方法」一个入口。
- 桌面端在页面内容区左侧显示二级导航。
- 移动端在页面顶部显示横向标签导航。
- 当前包含两个教程栏目：
  - `Codex 接入`：保留原 8 个步骤和 10 张截图。
  - `生图方法`：新增用户侧生图接入说明。

## 生图方法展示内容

页面展示给用户的信息包括：

- 29/39/59 元套餐已支持生图。
- 请求地址为 `https://api.aaccx.pw/v1`。
- 独立图片接口为 `POST /v1/images/generations`。
- 图片按实际分辨率消耗订阅日额度：
  - 1K：`$0.10 / 张`
  - 2K：`$0.20 / 张`
  - 4K：`$0.40 / 张`
- 示例请求使用 `Authorization: Bearer sk-xxxx` 占位，不展示真实 API Key。

## 修改边界

- 只修改前端 `/usage-guide` 页面和对应测试。
- 未修改后端、支付、订阅、兑换码、API Key、计费、分组配置或公网 nginx。
- 未把管理员分组 id、本地端口验证或运行态后台 API 细节展示给用户。

## 关键文件

- `frontend/src/views/user/UsageGuideView.vue`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

## 验证

- `pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
- `pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/router/__tests__/guards.spec.ts`
- `pnpm typecheck`
- `pnpm build`
- `git diff --check`

以上命令均已通过。构建输出仍包含项目既有的 Vite chunk 警告、Browserslist 数据提示和 Node deprecation 警告，不影响构建结果。
