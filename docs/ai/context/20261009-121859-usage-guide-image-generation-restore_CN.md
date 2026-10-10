# 使用方法页「生图方法」教程恢复并按现状重写（2026-10-09）

- 触发：管理员要求「使用教程页面里生图的教程之前下架了，看一下现在网站是怎么用图片生成的，把教程加回去」。
- 结论：**不是简单取消隐藏**。2026-07-07 写的旧教程已经和线上对不上，按现状重写后恢复显示。
- 改动文件：`frontend/src/views/user/UsageGuideView.vue`、`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`，另有本文档和 `AGENTS.md` 一条维护约束。**只改前端静态文案，不动后端、数据库和任何配置。**
- 上线方式：走 PR → 合并到 `main` → `build.yml` 构建镜像 → `deploy-mac` 自动部署到生产 Mac。上线前线上版本与 `fork/main` 同为 `b85fa42ac`（公开端点 `/api/v1/settings/public` 的 `version` 字段可核对），所以这次发布只带这一处改动。

## 下架经过

- 2026-08-05 提交 `765c193dd`「隐藏使用方法中的生图主题」：把主题数组改名 `allGuideTopics`，再用 `hiddenGuideTopicIds` 过滤掉 `image-generation`。
  主题数据留在源码里，所以这次可以原地恢复。
- 本次把这套隐藏机制整个去掉，数组名恢复为 `guideTopics`，直接 `.sort()` 按「更新于」倒序（逻辑与隐藏前一致）。

## 旧教程哪里已经不对

| 旧说法 | 现状 |
| --- | --- |
| 「29/39/59/79/99 元套餐已支持生图」 | 在售商品只有余额套餐和流量卡，生图与套餐档位无关，只看 Key 所属分组 |
| 「使用你已经生成的 API Key 即可直接请求」 | **不行**。分组 `allow_image_generation` 默认 `false`，只有分组 12 开着，其它分组的 Key 请求图片接口直接 `403 permission_error: Image generation is not enabled for this group` |
| 「按上游实际返回的 Token 用量和套餐有效倍率计费」 | 分组 12 已于 2026-10-04 改为渠道 14 **按张计费**（见 AGENTS.md 坑 35），不看 Token |
| 只提 `gpt-image-2` | 现在 4 个：`gpt-image-2`、`gpt-image-1.5`、`gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` |
| 示例带 `"response_format": "b64_json"` | gpt-image 系列固定返回 base64，OpenAI 官方对该系列传 `response_format` 会报未知参数；新示例一律不带 |

## 现状核对来源（都可复查）

- 公开端点 `https://aaccx.pw/api/v1/model-plaza`（零凭证）：全站只有分组 **#12「GPT生图1倍率」** 列有图片模型；`is_exclusive=false`（所有人可选）；
  4 个模型 `billing_mode=image`，1K `$0.134` / 2K `$0.201` / 4K `$0.268`，四个模型同价。
- 代码：
  - 分组权限：`service.GroupAllowsImageGeneration`，`ent/schema/group.go` 默认 `false`；
  - 尺寸分档：`service/image_billing_size.go`（`ResolveImageBillingSize`：优先取上游响应里每张图的 `size`，其次请求 `size`，都没有按 2K；最长边 ≤1024 为 1K、≤2048 为 2K、其余 4K）；
  - 未开放模型：`handler/no_account_error.go`（池里有账号但都不支持该模型 → `404 model_not_found`）；Images 处理器丢弃了渠道 `restrict_models` 标志，直接走账号选择；
  - 并发超限：`handler/openai_gateway_handler.go` 的 `acquireImageGenerationSlot` → `429 rate_limit_error: Image generation concurrency limit exceeded`；
  - 非流式保活 `gateway.image_nonstream_keepalive_interval` 默认 `0`（关），所以生成超过约 125 秒会遇到 Cloudflare 524（与 AGENTS.md 第七节那条一致）；
  - 参数解析：`service/openai_images.go`（JSON 图生图只认 `images[].image_url`，`file_id` 会 400；multipart 文件字段名 `image` / `image[...]`，局部修改用 `mask`）。
- 线上实测（无效 Key，仅验路径）：`POST api.aaccx.pw/v1/images/generations` 与 `/v1/images/edits` 都返回 `401 INVALID_API_KEY`，两个入口都在。
- 创建 Key 的界面路径（侧栏「API 密钥」→「创建密钥」→ 弹窗「分组」下拉）与「在密钥列表点分组一栏可改组」均对照 `KeysView.vue` 和现有截图确认。

## 新教程内容（7 节）

先创建生图专用的 API Key → 可用模型与计费 → 接口地址（表）→ 文字生图（curl）→ Python 示例（保存图片）→ 图生图（JSON / multipart）→ 常见报错（7 条）。
`updatedAt` 取 `2026-10-09`，所以它排在导航第一位、**成为页面默认打开的主题**（页面按更新日期倒序，原来是 DeepSeek Harness）。

## 刻意的取舍

- **不写具体单价**，只写分档规则并指向模型广场：教程里的数字是「标准成本」，用户实扣还会再乘分组倍率和隐藏最终倍率，写出来既容易误读，也会在渠道 14 改价后过期。教程全文**没有出现隐藏倍率**。
- **不提批量生图和异步生图**：`batch_image` 开关按 AGENTS.md 第五点五节必须保持关闭；`/v1/images/*/async` 要对象存储开着才可用，生产状态没确认。
- **不复用现有截图**：现有「创建密钥」截图选的是 GPT 分组，会误导；用文字描述路径，避免界面改版后截图过期。
- **报错用分条文字，不用 `errorRows` 表格**：该表 `min-width: 58rem`，实测在 1280 宽视口下「当前含义」那列被裁掉 306px，要横向拖动才读得到。接口表（`min-width: 44rem`，溢出 82px）沿用「规范使用」同款，保留。
- 建议「新建专用 Key」而不是把旧 Key 改选成生图分组：分组 12 只有图片模型，改选后该 Key 原来的聊天用途就失效了。

## 验证

- `vitest run UsageGuideView.spec.ts`：4 个全过。新增一个**真正挂载页面**的用例（桌面导航 + 移动端标签都能看到「生图方法」，点开后含分组名、4 个模型、三个接口、报错码）。
  做了变异验证：临时把主题重新过滤掉，该用例失败而另外三个「读源码字符串」的用例仍然通过——说明单靠字符串断言拦不住「换种方式又隐藏」。
- `eslint`（两个文件）和 `vue-tsc --noEmit`（全项目）真实退出码均为 0、无输出。注意：别写 `cmd | tail` 再取 `$?`，那是 `tail` 的退出码（第六节第三条）。
- 浏览器渲染（mock 后端 + `frontend-dev`）：1280 宽无页面级横向溢出；375 宽无页面级横向溢出，代码块在块内自己滚动，所有卡片右边距一致。临时 mock、`frontend/.env.local`、预览服务均已清理。

## 没验证的（请知悉）

- **没有用真实 Key 跑过任何生图请求**。建临时 Key 要管理员在对话里明确授权，且渠道 14 的真实网关扣费核对本来就还没做（AGENTS.md 第七节）。
  示例的请求形态来自代码解析和旧教程；`data[0].b64_json` 的返回形态、`n>1`、`mask` 的上游行为属于代码推断，未实测。
- 「各模型支持的尺寸以上游为准」「没出图的失败请求不扣费」「被内容审核拦截不扣费」来自代码阅读（失败路径在记账之前返回），没有用 `usage_logs` 对过。

## 以后怎么维护

下面任何一项变了，都要回来改 `UsageGuideView.vue` 里的 `image-generation` 主题（搜 `GPT生图1倍率`、`gpt-image-`）：分组 12 改名、白名单增减模型、渠道 14 调整分档或价格口径、`allow_image_generation` 的授权方式。
旧版就是因为没人同步才留下了已经不存在的套餐档位和 Token 计费说法。

## 上线与回滚

- 本次是从最新 `fork/main` 另开的临时 worktree 里提交的：主工作区的 `AGENTS.md` 与 `main` 版不是同一个版本（带着其它会话未提交的改动，且 `main` 上没有坑 35），所以维护约束是直接加在 `main` 版 `AGENTS.md`「四、业务规则 → 其他」里的。
  验证也在该 worktree 里按 `main` 的锁文件重新装依赖后跑的（`main` 比主工作区当前分支多了依赖，借主仓 `node_modules` 做类型检查会得到无关的缺依赖报错）。
- 上线会重启一次应用容器，进行中的流式请求会被中断，属于所有部署的共同代价。
- 回滚：页面文案是纯静态的，回滚 = revert 本次合并提交再走一次部署，或在 Mac 上 `~/sub2api/deploy-mac.sh sha-b85fa42ac…`（上一版完整 sha 见 `git log`）。
- 部署后核对：公开端点 `/api/v1/settings/public` 的 `version` 应变成 `main-<合并提交 sha>`；该页面需要登录，无法零凭证直接打开，可去线上前端静态资源里搜 `GPT生图1倍率` 确认新教程已随镜像发布。
