# main 分支“已合并但内容不在”的排查记录

时间：2026-06-24 21:06:39 +0900

## 结论

- 当前本地 `main` 为 `1672e097 fix: clarify image guide and add redeploy helper`，领先 `origin/main` 41 个提交。
- `codex/usage-guide-99-plan-main-20260624` 被 Git 判定为已合并，是因为它创建自 `origin/main` 的 `85a3b122`，本身不包含 99 元套餐改动。
- 真正包含 99 元教程文案的分支是 `codex/usage-guide-99-plan-20260624`，HEAD 为 `9caf7efe`，该分支没有合并进当前 `main`。
- 当前 `main` 的 `/usage-guide` 源码和公网 chunk 已包含 `29/39/59/99`、`图生图`、`/images/edits`、`gpt-image-2` 和 `Trae 接入`；这部分不是公网静态包未更新。
- 公网购买页没有 99 元套餐的根因不是前端发布失败，而是运行态数据库没有 99 元在售订阅计划，也没有对应 99 元 group。

## 关键证据

- `git branch --merged main` 包含 `codex/usage-guide-99-plan-main-20260624`。
- `git reflog show codex/usage-guide-99-plan-main-20260624` 显示该分支仅有一条记录：`Created from origin/main`。
- `git branch --no-merged main` 包含 `codex/usage-guide-99-plan-20260624`、`codex/usage-guide-image-edit-20260624`、`codex/deploy-runtime-scripts-20260624`。
- `git branch --contains 9caf7efe` 只返回 `codex/usage-guide-99-plan-20260624`，不包含 `main`。
- `git branch --contains 27bd8090` 只返回两个图生图旧分支和 99 文案分支，不包含 `main`。
- `git branch --contains 4e5e4c9e` 返回 `main`；当前 `main` 的使用教程内容来自后续 `restore usage guide page` 与 `1672e097` 的手工恢复/修正文案，不是原样合并旧分支。
- 公网 `https://aaccx.pw/assets/UsageGuideView-rcRYJt8N.js` 包含 `29/39/59/99 元套餐已支持生图和图生图`、`Trae 接入`、`https://api.aaccx.pw/v1/images/edits`。
- 运行态 DB 查询结果显示 `subscription_plans` 只有：
  - `29 元订阅池`
  - `39 元订阅池`
  - `59 元订阅池`
- 运行态 `groups` 只有 `codex-pool-19-usd`、`codex-pool-29-usd`、`codex-pool-49-usd` 和本机自用无限额分组；没有 99 元或 89 USD 日限额分组。

## 原因拆解

1. 分支名误导：`usage-guide-99-plan-main` 看起来像“99 计划合并 main 用分支”，但它实际上是从 `origin/main` 新建的空内容分支。
2. patch 与内容不是一回事：`git cherry` 显示旧 99 和图生图 commit 没有 patch 等价进入 `main`，但当前 `main` 已经通过另一个 restore commit 手工恢复了相近且更新的前端文案。
3. 99 元“套餐”需要运行态配置：现有 99 改动只覆盖使用教程文案，没有创建 `subscription_plans` 记录，也没有创建/绑定新 `groups`，所以购买页不会出现 99 元套餐。
4. `codex/deploy-runtime-scripts-20260624` 不应直接合并：它相对当前 `main` 有大量旧树差异，直接合并会回滚/删除许多当前 main 内容。当前 main 仅包含 `deploy/redeploy-sub2api-image.sh`，不包含该旧分支里的 restart 脚本和测试脚本。

## 建议

- 不要再按 `codex/usage-guide-99-plan-main-20260624` 判断 99 元功能是否已合并；这个分支可以视作误导性空分支。
- 不要直接 merge `codex/usage-guide-image-edit-20260624` 或 `codex/deploy-runtime-scripts-20260624`，需要时应 cherry-pick 精确文件或重新在当前 `main` 上实现。
- 若要让官网购买页出现 99 元套餐，需要单独做运行态配置：创建新 group、设置日限额和图片价格、创建 `99 元订阅池` 的 `subscription_plans` 记录，并验证 `/purchase`。
