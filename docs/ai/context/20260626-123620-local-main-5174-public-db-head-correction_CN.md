# 本地 5174 启动 HEAD 更正记录

## 更正

前一份结果文档记录启动前初查到的 `main` HEAD 为 `0df3031508f04b73bf6c92aafd4d37b34ad73f48`。最终完成验证时，当前 worktree 的 `main` HEAD 为：

```text
60e9da8d0cbc588dbf64b188ab08b6aad594a1b7
```

最近提交：

```text
60e9da8d0 docs: 记录 personal main 同步结果
2a2ea445c docs: 记录 personal main 同步计划
0df303150 feat: 订阅页统一展示流量包用量
```

## 当前有效状态

- 本地 5174 服务运行的是 `/Users/wujianxiang/CodeSpace/sub2api` 当前工作区文件。
- 当前分支：`main`
- 当前 HEAD：`60e9da8d0cbc588dbf64b188ab08b6aad594a1b7`
- 当前分叉状态：`main...origin/main [ahead 50, behind 47]`
- 工作区 tracked 改动：无。

## 仍然成立的验证结果

- `screen -ls` 显示 `71485.sub2api-frontend-5174 (Detached)`。
- `lsof -nP -iTCP:5174 -sTCP:LISTEN` 显示 `node` 正在监听 `*:5174`。
- `curl -I http://127.0.0.1:5174` 返回 `HTTP/1.1 200 OK`。
- `curl http://127.0.0.1:5174/api/v1/settings/public` 返回 `code=0`、`message=success`。
- `curl http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
