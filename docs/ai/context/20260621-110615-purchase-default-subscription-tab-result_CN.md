# /purchase 默认订阅页签调整结果

## 结果

- `/purchase` 页面默认打开后展示订阅内容。
- 页签顺序改为「订阅」在左、「充值」在右。
- 余额充值禁用时仍沿用原逻辑，不展示充值页签。
- 未修改支付接口、订阅接口、订单创建、数据库、路由或计费逻辑。

## 变更文件

- `frontend/src/views/user/PaymentView.vue`
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`

## 验证

- `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/__tests__/HomeView.spec.ts src/__tests__/visualThemeSource.spec.ts`：3 个测试文件通过，12 个测试通过。
- `pnpm build`：构建通过，输出到 `backend/internal/web/dist`；仅出现既有 Vite 动态导入、大 chunk 和 Browserslist 数据过期警告。
- `git diff --check`：通过。
- `curl -I http://127.0.0.1:5174/purchase`：返回 `HTTP/1.1 200 OK`。

## 发布状态

- 本地前端预览服务仍可通过 `http://localhost:5174/purchase` 查看。
- 尚未推送代码，尚未重新部署公网版本。
