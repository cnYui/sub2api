# UsageGuide 恢复校验与主分支合并设计

## 背景

- 当前工作区位于 `/Users/wujianxiang/CodeSpace/sub2api`，分支为 `codex/deploy-runtime-scripts-20260624`，不是 `main`。
- 本次待处理的前端改动是恢复并保留普通用户 `UsageGuide` 页面，同时把 99 元套餐对应的图生图说明带回页面。
- 用户已明确要求：
  - `UsageGuide` 页面必须保留；
  - 需要 review 当前改动；
  - 需要把确认无误的改动合并到本地 `main`；
  - 希望本地工作区更干净。

## 当前发现

### 1. UsageGuide 当前不具备可合并状态

- `frontend/src/views/user/UsageGuideView.vue` 已导入 `Trae` 四张截图和 `traeSetupSteps`，但 `guideTopics` 中没有 `Trae 接入` 栏目。
- 当前 `frontend/src/assets/usage-guide/` 只恢复了 10 张 Codex 接入截图，没有恢复 `Trae` 的 4 张截图资源。
- 因此当前页面和线上截图不一致，并且前端构建存在资源缺失风险。

### 2. 当前 worktree 不是主分支工作区

- 当前工作区分支是 `codex/deploy-runtime-scripts-20260624`。
- 本地 `main` 指向另一条提交，不在当前 worktree 上。
- 不能直接把当前整个分支 merge 到 `main`，否则会把本次需求无关的历史提交一起带入。

### 3. 本地还有临时备份文件污染 git status

- 根目录存在多个 `.tmp-*`、`*.dump`、`*.sqlite*` 文件。
- 这些文件不是这次功能的一部分，只会影响“干净可提交状态”的判断。

## 目标

1. 让 `UsageGuide` 页面恢复到可发布状态：
   - 保留 `Codex 接入`
   - 保留 `生图方法`
   - 恢复 `Trae 接入`
   - 生图说明保持“使用方法 2”的图生图版本，并包含 99 元套餐
2. 让测试能够覆盖这次恢复，避免以后再次把 `Trae` 栏目弄丢。
3. 把本地临时垃圾与本次代码改动分开处理。
4. 只把确认无误的 UsageGuide 相关改动合并进本地 `main`。

## 方案选择

### 方案 A：继续保留当前两栏目版本，直接 merge

- 优点：最快。
- 缺点：与用户截图和线上现状不一致，且缺失资源会导致构建风险。

不采用。它会把明显错误带进主分支。

### 方案 B：按历史提交完整恢复 Trae 教程，再叠加 99 元图生图说明

- 优点：
  - 与历史功能演进一致；
  - 改动范围最小；
  - 能复用已有截图与页面结构；
  - 便于后续 cherry-pick 到 `main`。
- 缺点：需要补 4 张截图，并更新测试。

采用此方案。

## 设计结论

- 页面层只做最小恢复：
  - `UsageGuideView.vue` 增加 `Trae 接入` 主题；
  - 保留现有图生图文案与 99 元套餐说明；
  - 不改路由结构和现有导航权限策略。
- 测试层先补红灯：
  - 新增对 `Trae 接入` 主题、4 张截图说明和资源存在性的断言；
  - 再修实现，跑通过。
- Git 集成层不直接 merge 整个当前分支：
  - 先在当前分支形成一条只包含 UsageGuide 修复的提交；
  - 再切到本地 `main`，用 `cherry-pick` 合入该提交；
  - 这样不会把 `feat: add sub2api redeploy and restart scripts` 一并带进 `main`。
- 本地清理层优先删除这批临时备份文件，不把它们纳入提交范围；如仍需长期规避，再单独评估是否补 `.gitignore` 规则。
