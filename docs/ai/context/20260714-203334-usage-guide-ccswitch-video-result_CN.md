# /usage-guide CCSwitch 视频教程结果

## 完成内容

- 在 `/usage-guide` 新增独立「CCSwitch 视频教程」栏目，位于现有「Codex 接入」之后。
- 栏目说明为「完整演示使用 CCSwitch 接入中转站，解决 99% 常见的连接不上、断连问题。」
- 使用原生视频播放器，启用 `controls`、`playsinline` 和 `preload="metadata"`，不自动播放，并提供视频文件回退链接。
- 视频和封面使用稳定公开路径，不新增路由、播放器依赖或外部视频托管。

## 视频压缩

- 原文件：`/Users/wujianxiang/Downloads/112_raw.MP4`。
- 原始规格：2476x1440、30 fps、H.264，时长 115.750998 秒，43,137,698 字节。
- 输出视频：`frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4`。
- 输出规格：1920x1116、30 fps、H.264 High、`yuv420p`、AAC，时长 115.750998 秒，3,178,459 字节。
- 压缩后体积约为原文件的 7.37%，减少约 92.63%。
- 已启用 MP4 `faststart`；FFmpeg 在编码结束时确认将 `moov` atom 移到文件开头。
- 封面：`frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp`，1280x744，37,912 字节。
- `ffmpeg -v error -i ... -f null -` 完整解码通过，无媒体损坏错误。

## TDD 与验证

- RED：`UsageGuideView.spec.ts` 新增栏目、播放器和静态资源契约后，10 项测试中 3 项按预期失败，原因分别为栏目、`video` 类型和资源缺失。
- GREEN：实现后目标测试 10/10 通过。
- `pnpm -C frontend typecheck` 通过。
- `pnpm -C frontend build` 通过；构建产物包含相同字节大小的视频和封面。仅出现仓库既有动态导入、Browserslist 数据和 chunk 大小警告。
- `go test -C backend -count=1 -tags=embed ./internal/web` 通过。
- `git diff --check` 通过。
- 用户明确表示不需要浏览器操作，将自行验证页面，因此未继续浏览器视觉或播放交互检查。

## Git 集成

- 设计提交：`59fd11e7c`。
- 计划提交：`8efe7dae9`。
- 计划命令修正：`0edebe51a`。
- 功能提交：`20ec3c9d6`。
- 共享功能分支随后出现两条其他任务的 LOCAL Key 文档提交；集成时创建临时干净指针，仅将上述四个本任务提交快进到本地 `main`，未夹带无关提交。
- 集成前 `personal/main...main` 为 `0 7`，个人远端没有独立分叉。

## 范围

- 未修改后端业务、数据库、Redis、Nginx、容器或公网运行态。
- 未删除或替换现有截图教程。
- 未推送 `origin`。
- 本文档提交后将在本地 `main` 复验，并同步到 `personal/main`。
