# personal/main 同步计划

## 背景

- 用户要求把当前本地最新 `main` 同步到 GitHub 上自己的仓库。
- 当前仓库存在两个远端：
  - `origin`: `https://github.com/Wei-Shaw/sub2api.git`
  - `personal`: `https://github.com/cnYui/sub2api.git`
- 用户语义中的“我自己的仓库”按现有远端判断为 `personal`。

## 当前状态

- 当前分支：`main`
- 工作区：开始同步前无未提交变更。
- `git fetch --all --prune` 后：
  - `personal/main` 停在 `4e5e4c9ee feat: restore usage guide page`
  - 本地 `main` 比 `personal/main` 多 9 个提交
  - `personal/main` 没有本地缺失的提交，因此可普通推送快进
- `origin/main` 比本地 `main` 有新的上游提交，本次不把这些上游提交合入本地，避免把主仓最新变更混入用户明确要求的“当前本地最新 main”同步动作。

## 方案

1. 确认本地 `main` 与 `personal/main` 的差异只包含本地领先提交。
2. 新建本上下文计划文档，记录同步目标、风险和取舍。
3. 新建结果文档并提交，保证 AGENTS 要求的同步上下文不会停留为未提交文件。
4. 将本地 `main` 推送到 `personal/main`。
5. 推送后再次 fetch 并确认 `main` 与 `personal/main` 指向同一提交。

## 取舍

- 不强推：当前 `personal/main` 可快进，普通 push 更安全。
- 不合并 `origin/main`：`origin/main` 已经前进且与本地有分叉，合入属于另一个决策，会引入远端上游代码变化和潜在冲突。
- 会新增文档提交：这是为了满足项目 AGENTS 的上下文沉淀要求，并让同步完成后工作区保持干净。
