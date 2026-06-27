# Codex `logs_2.sqlite` TRACE 高频写盘检测

## 结论

- `~/.codex/logs_2.sqlite` 当前确实存在持续写入，且主要由 TRACE 日志贡献。
- 高频写入不是单纯文件历史遗留：15 秒采样内新增约 426 行日志，净行数只增加 44 行，说明当前 Codex app-server 一边写入一边清理旧行，形成持续写盘。
- 最大磁盘体积来源是 `TRACE codex_client::transport`，该 target 历史累计约 514.14 MiB，单条最大约 6.1 MiB；最近 1 小时仍写入约 18.46 MiB。
- 不建议在诊断结果文档中记录日志正文，避免误带 API Key、请求体或其它敏感内容。

## 关键证据

- 文件状态：
  - 主库：`~/.codex/logs_2.sqlite` 约 806 MiB
  - WAL：`~/.codex/logs_2.sqlite-wal` 约 6.1 MiB，mtime 持续更新
  - SHM：`~/.codex/logs_2.sqlite-shm` 32 KiB
- 当前持有者：
  - `codex` PID 3250 持有主库、WAL、SHM
  - 命令为 `/Applications/Codex.app/Contents/Resources/codex app-server --analytics-default-enabled`
- 表结构：
  - `logs(id, ts, ts_nanos, level, target, feedback_log_body, ..., estimated_bytes)`
  - SQLite 使用 WAL 模式，`wal_autocheckpoint=1000`
- 总体分布：
  - TRACE：44,204 行，约 550.48 MiB
  - INFO：39,441 行，约 35.49 MiB
  - DEBUG：16,236 行，约 17.62 MiB
- 最近 10 分钟：
  - TRACE 940 行，约 9.58 MiB
  - INFO 531 行，约 0.41 MiB
  - DEBUG 130 行，约 0.18 MiB
- 15 秒增量采样：
  - `TRACE codex_api::sse::responses`：126 行
  - `INFO codex_otel.log_only`：125 行
  - `TRACE codex_app_server::outgoing_message`：111 行
  - `TRACE codex_client::transport`：1 行，约 297.7 KiB

## 判断

`logs_2.sqlite` 变大和当前写盘压力的根因是 Codex Desktop app-server 处于较高日志详细度，尤其 TRACE 级别的 transport/SSE/outgoing message 类日志持续入库；其中 transport 日志因为会记录大体积内容，是空间占用主因。SQLite 页统计显示主库约 806 MiB 中有约 134 MiB freelist，说明清理旧行后空间未完全归还给文件系统。

## 后续可选动作

- 若只想降低写盘，优先关闭或降低 Codex app-server 的 TRACE 日志级别。
- 若只想回收磁盘空间，需要在 Codex 退出并备份后做 SQLite 回收，例如 VACUUM 或重建日志库；不要在进程持有 WAL 写句柄时直接删库。
- 若要保留可追溯性，可先导出最近必要窗口，再清理旧 TRACE 日志。
