# 删除 sub2api-local-redeploy skill 计划

## 背景

用户要求删除本地 Codex skill：`/Users/wujianxiang/.codex/skills/sub2api-local-redeploy/SKILL.md`。

该 skill 用于重建并替换 Sub2API 本地/公网应用容器，属于高风险运行态操作入口。当前请求不是执行重部署，而是移除该自动化入口。

## 方案

- 删除整个目录 `/Users/wujianxiang/.codex/skills/sub2api-local-redeploy`，避免只删除 `SKILL.md` 后留下无效的 `agents/openai.yaml`。
- 不执行该 skill 中的任何 Docker、Compose、curl 或部署命令。
- 不读取、打印或记录 `deploy/.env.scheme-a.local`。
- 删除后用文件系统检查确认目录不存在。

## 风险

- 删除后 Codex 不再自动识别 `$sub2api-local-redeploy`。
- 如未来还需要同类自动化，需要重新创建 skill 或从备份恢复。
