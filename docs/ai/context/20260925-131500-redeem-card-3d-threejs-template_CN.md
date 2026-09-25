# 兑换卡 3D：按站长的 Claude Design 模板改用 three.js（2026-09-25）

接 `20260923-215000-redeem-cards-3d-admin-page_CN.md`。那一版的 3D 是 CSS 自己拼的，因为当时读不到站长的模板。

## 模板怎么拿到的

- 模板在站长的 Claude Design 项目「中转站兑换码卡片设计」里：`兑换卡 3D.html`（卡片场景）+ `three-d-stage.js`（Claude Design 自带的 three.js 舞台组件），
  贴图 `textures/{black,white}-{front,back}.png` 是把平面成品卡截成的 3424×2160 静态图。
- 站长的 Chrome 登录着 claude.ai。`.js` 文件在项目「All project files」里点开是代码视图，有 Copy 按钮；
  HTML 页面没有代码视图，只能 Share → Export → Project HTML → **Project archive**（zip，即时、不耗用量；另一个 Standalone HTML 要调 Claude、占用量）。
- Chrome 扩展会拦下 `javascript_tool` 返回的文件内容（报「Cookie/query string data」），别想着用脚本读源码。

## 模板内容 → 代码

全部落在 `frontend/src/components/redeemCard/cardStage.ts`，数值照抄：

| 模板 | 值 |
| --- | --- |
| 卡片 | CR80 85.6×54mm，圆角 3.2mm；`ExtrudeGeometry`，倒角 0.15mm（`bevelSegments: 3`、`curveSegments: 14`） |
| 端面 UV | 正反面各自铺满贴图，背面 `u → 1-u` 翻回来 |
| 侧边 | 顶点色 `#2b8a99 → #3a3f6e → #8e2f72`，`t = ((x+W/2)/W + (H/2-y)/H)/2` |
| 材质 | 卡面 `MeshPhysicalMaterial` 粗糙度 0.55、清漆 0.25/0.4；侧边粗糙度 0.45、金属度 0.1、清漆 0.5/0.3 |
| 渲染 | `NeutralToneMapping`、曝光 1、抗锯齿、像素比 ≤2、贴图各向异性拉满 |
| 灯光 | 半球光 0.7（`#ffffff`/`#d8d2c4`），主光 1.6 在 (4,7,5) 投影 2048，补光 0.4 在 (-5,3,-4)，地面 `ShadowMaterial` 0.18 |
| 相机 | FOV 45，near 0.002；取景方向 (0.2, 0.14, 1)，横向留 1.12、纵向留 1.3 |
| 控制 | `OrbitControls` 阻尼 0.08，距离 0.015–0.6，开场自转 1.2、一碰就停 |
| 翻面 | 相机水平转 180°，700ms 缓入缓出 |

和模板不一样的地方：

- 厚度 0.6mm（站长要求；模板滑杆默认 1.2mm，最小 0.6mm）。
- 关掉平移：页面上没有「正视」按钮，平移走了回不来。后台预览再关掉缩放，免得滚轮滚页面时缩放卡片。
- 模板的 `PCFSoftShadowMap` 在 r184 已废弃，渲染时会被换成 `PCFShadowMap`，这里直接写后者，效果一样。
- 贴图不是静态图：运行时把 `RedeemCardFace.vue` 的两面 DOM（排在视口外）用 `html2canvas-pro` 截成 2 倍图。
  圆角外透明的像素先垫一层描边渐变，否则 3D 端面圆角略大，四角会露出黑点。
- 复制兑换码的绿光是贴在背面上的一层透明平面（只画面板外圈），轻点位置用射线求交后的 UV 换算成卡面像素，再判断是否落在兑换码面板里。
- 不支持 WebGL2 或初始化失败时退回平面卡：轻点翻面、背面轻点兑换码复制。
- 公开页背景换成模板舞台的 `#e9e8e4`（原来黑卡配深色渐变、白卡配浅色渐变），地面阴影要落在浅底上才看得见。

## 分包

`three`（541KB）和 `html2canvas-pro`（259KB）在 `vite.config.ts` 里单独分到 `lib-three`、`lib-html2canvas`，
只出现在动态导入的 `__vite__mapDeps` 里，首页不加载。

踩坑：手动 `npx vite build` 时 Vite 优先读 `frontend/vite.config.js`，那是 `vue-tsc -b` 生成的旧产物（已 gitignore），
结果 three.js 全进了 `lib-misc`（1063KB）。`npm run build` 会先 `vue-tsc -b` 重新生成它，Docker 里也是这样，所以线上没问题。

## 验证

- 把导出的模板在本地起静态服务，和我们的页面放同一视口（846×522）、同一相机位置逐一对比：黑色正面、侧面、背面、白色正面都一致。
  唯一的差别是字体：模板贴图用 Noto Sans SC，我们用系统中文字体（Windows 上是微软雅黑，iPhone 上是苹方）。
- 本地 mock 接口走了一遍：轻点翻面、背面轻点兑换码复制（拦截 `writeText` 确认复制的是库里原文 `7F3K9QXA2M8DTC01`）、拖动旋转不误翻面、
  「已兑换」盖章、手机 375 宽取景、后台 3D 预览改配色和面值后贴图跟着重截。
- `vitest run` 全量 208 个文件 1445 个用例通过，`vue-tsc`、ESLint、构建通过。新增 `cardStage.spec.ts` 钉住厚度、材质分组、背面 UV 不镜像、侧边渐变端点色。
- 没在真实 iPhone / 微信里跑过。背面比正面暗，是模板灯光的原样（主光在正面一侧）。
