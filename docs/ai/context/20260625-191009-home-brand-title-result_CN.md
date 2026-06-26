# /home 首页品牌文案调整结果

## 变更

- `/home` 默认公共设置标题改为 `天才程序员小站`。
- `/home` 在公共设置返回空 `site_subtitle` 时不再回退展示 `AI API Gateway` 或旧英文副标题。
- 后端公共设置和系统设置缺省值统一为 `site_name=天才程序员小站`、`site_subtitle=`。
- 保留前端全局默认品牌常量、浏览器标题、管理后台表单初始值不变，避免超出“只改 /home 页面内容”的范围。

## 修改文件

- `backend/internal/service/setting_service.go`
- `backend/internal/handler/setting_handler_public_test.go`
- `backend/internal/server/api_contract_test.go`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/__tests__/HomeView.spec.ts`

## 验证

- `pnpm --dir frontend test:run src/constants/__tests__/branding.spec.ts src/views/__tests__/HomeView.spec.ts` 通过。
- `go test -tags unit ./internal/handler -run 'TestSettingHandler_GetPublicSettings_(UsesDefaultHomeBranding|ExposesForceEmailOnThirdPartySignup|ExposesWeChatOAuthModeCapabilities)' -count=1` 通过。
- `go test -tags unit ./internal/server -run TestAPIContracts -count=1` 通过。

## 注意

如果运行中的数据库已经保存了旧 `site_name=Sub2API` 或 `site_subtitle=Subscription to API Conversion Platform`，代码默认值不会覆盖已存在配置；需要通过管理后台或数据库把对应配置更新为 `天才程序员小站` 和空副标题。
