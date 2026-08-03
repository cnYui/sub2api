# 本地前端品牌显示修正

## 背景

运行时公共设置的 `site_name` 仍为 `Sub2API`，会覆盖此前修改的默认值，导致侧栏与主页仍显示旧品牌。

## 修改

- 侧栏固定显示“天才程序员小站”。
- 主页标题固定显示 `Genius Programmer Hub`，保留现有英文副标题和站点图标。
- 浏览器标签标题固定使用“天才程序员小站”。
- 品牌常量集中在 `frontend/src/utils/branding.ts`，未改动后端设置与持久化数据。

## 验证

- `pnpm typecheck` 通过。
- `sub2api-official-18082` 使用镜像 `sha256:7ed3cab530e984be829fe192cf00592e0e1c3432ba3ecef59db3d1f3f93b4497`，状态 `healthy`。
- `GET http://127.0.0.1:18082/health` 返回 HTTP 200。
- 浏览器实测主页、管理侧栏及页面标签均显示新品牌文案。
