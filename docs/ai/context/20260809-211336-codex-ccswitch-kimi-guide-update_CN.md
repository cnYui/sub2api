# Codex 通过 CC Switch 接入 KIMI 分组教程更新

## 需求

更新登录后 `/usage-guide` 的“Codex 接入”主题，加入用户提供的最新版 CC Switch 配置流程。教程需要一张图片对应一句操作说明，共 10 步；第 3、4 步按用户指定交换上传图片的原始顺序。

## 实现决策

- 继续复用 `UsageGuideView.vue` 现有的 `GuideStep` 数据结构和步骤渲染，不新增平行页面组件。
- Codex 教程步骤固定为：创建包含 KIMI 分组的密钥、打开 CC Switch 并新建凭证、填写 API Key 与请求地址、确认填写结果、打开高级选项、获取模型列表、添加并映射模型、确认映射、保存、重启 Codex 验证自定义模型。
- 图片按 `57aacca6 → a3bd247c → e747d4e0 → 3368cc64 → cb444c3e → e238608d → bab159b0 → 76bf9ae → 4b0c511d → fbf29360` 使用，并复制为 `codex-ccswitch-step-01.png` 至 `codex-ccswitch-step-10.png`。
- 第 6、9 张截图原图包含完整 `OPENAI_API_KEY`，入库前以 `sk-xxxx` 替换密钥值；文案和其它截图不保存真实凭证。
- Codex 主题更新时间更新为 `2026-08-09`，页面说明改为 KIMI 分组、CC Switch 模型映射和 Codex 自定义模型结果。

## 验证

- `UsageGuideView.spec.ts` 资源存在性清单覆盖 10 张新截图。
- 计划运行前端定向 Vitest、类型检查和生产构建；本次只改前端静态资源与页面，不发布公网容器。
