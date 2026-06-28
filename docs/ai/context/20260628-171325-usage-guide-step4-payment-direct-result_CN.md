# /usage-guide 第 4 步支付直用改造结果

## 改动

- 将 `/usage-guide` Codex 接入第 4 步标题从“完成支付后，悠一会给你一个兑换码”改为“完成支付”。
- 将第 5 步标题从“兑换成功后，去 API Key 页面生成密钥”改为“支付完成后，去 API Key 页面生成密钥”。
- 删除第 4 步旧的两张兑换码流程配图引用。
- 新增并引用用户提供的新支付确认截图：`frontend/src/assets/usage-guide/step-04-confirm-payment.png`。
- 删除旧资源文件：
  - `frontend/src/assets/usage-guide/step-04-payment-submitted.png`
  - `frontend/src/assets/usage-guide/step-04-redeem-code.png`
- 更新 `UsageGuideView.spec.ts`，Codex 接入截图数从 10 调整为 9，教程截图资源数从 14 调整为 13，并断言旧兑换码文案不再出现。

## 原因

当前支付成功后会自动履约，用户不再需要管理员发放兑换码；继续展示兑换码流程会误导新用户。

## 验证

- 已先运行更新后的单测并确认失败，失败原因是旧标题仍存在且新图资源缺失。
- 修改后运行：
  - `pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts`
- 结果：6 个测试全部通过。

## 注意

- 本次只修改 `/usage-guide` 内容和截图资源，不改支付、订阅、订单、履约或路由逻辑。
