# Usage Guide Trae 接入结果

## 结果

- 已在 `/usage-guide` 新增「Trae 接入」栏目，复用现有步骤式说明页面样式。
- 已新增 4 个 Trae 配置步骤：
  1. 点击“添加模型”
  2. 选择“自定义配置”
  3. 填入 `https://api.aaccx.pw/v1`、自己的 API Key 和 `gpt-5.5`
  4. 点击“自定义模型”里的 `gpt-5.5` 即可使用
- 已把用户提供的 4 张截图复制为前端静态资源：
  - `frontend/src/assets/usage-guide/trae-step-01-add-model.png`
  - `frontend/src/assets/usage-guide/trae-step-02-custom-config.png`
  - `frontend/src/assets/usage-guide/trae-step-03-fill-url-key.png`
  - `frontend/src/assets/usage-guide/trae-step-04-select-model.png`
- 已新增 `GuideStep` 共享类型，避免多个步骤数组联合后模板访问 `imagePosition` 报类型错误。

## 未改动

- 未修改后端、计费、API Key 创建逻辑、路由或公网配置。
- 未在文档和代码中写入任何真实 API Key。

## 验证

```bash
pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
```

结果：5 个测试通过。

```bash
pnpm --dir frontend typecheck
```

结果：通过。

```bash
pnpm --dir frontend build
```

结果：构建通过，新增 Trae 图片已输出到前端构建产物；构建中仍有既有动态/静态导入与 chunk 大小提示。
