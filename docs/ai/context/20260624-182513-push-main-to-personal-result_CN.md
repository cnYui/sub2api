# 推送本地 main 到 personal 远端结果

## 操作

- 当前工作区：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`main`
- 推送目标远端：`personal`
- 推送命令：`git push personal main:main`

## 结果

- 推送成功。
- 远端返回：
  - `25ebb8a9..4e5e4c9e  main -> main`
- 本地 `HEAD`：
  - `4e5e4c9ee162ae7a7f24e68372c27d1850ee5adb`
- 远端 `personal/main`：
  - `4e5e4c9ee162ae7a7f24e68372c27d1850ee5adb`

说明本地 `main` 与 `personal/main` 已对齐。

## 当前状态

- `git status --short --branch`
  - `## main...origin/main [ahead 39]`
- 当前工作区无未提交修改
- 这里只是推送到了个人仓库 `personal`，没有推送到上游 `origin`
