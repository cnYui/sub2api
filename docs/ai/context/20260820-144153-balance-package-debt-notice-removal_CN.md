# 余额套餐欠费提示移除

时间：2026-08-20

## 结论

2026-08-19 的连续结算规则已经将余额套餐欠费处理改为自动连续刷新：套餐不再因欠费暂停，下一周额度到账时先抵扣普通余额欠费，剩余额度继续可用。历史有效 `debt_paused` 数据也由迁移恢复为可刷新状态。

因此，订阅页原有“首周额度不足以抵销欠费，后续额度已暂停，请联系管理员”提示已过时，本次从中英文文案和页面模板中移除。兼容历史 `debt_paused` 状态时，若接口仍返回 `next_credit_at`，统一展示“下次刷新”，不再显示“原计划刷新时间”。

## 修改范围

- `frontend/src/views/user/SubscriptionsView.vue`：删除欠费暂停黄色提示；`next_credit_at` 只要存在就展示正常下次刷新。
- `frontend/src/i18n/locales/zh/misc.ts`、`frontend/src/i18n/locales/en/misc.ts`：删除不再使用的提示文案。
- `frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`：更新历史状态兼容测试，锁定提示不出现且仍展示刷新时间。

## 验证

- `pnpm vitest run src/views/user/__tests__/SubscriptionsView.spec.ts`：3 个测试全部通过。
- `pnpm vue-tsc --noEmit`：通过。
- `git diff --check`：通过；仅有既存 `AGENTS.md` 换行格式提示。

本次只改前端展示，不修改后端计费、迁移或数据库数据。
