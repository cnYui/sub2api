# Security Scan 转红：Go 1.26.9 + x/net v0.60.0 + axios 1.20.0 + vue 3.5.43（2026-10-10）

- 触发：给生图教程开 PR（#64）时，`Security Scan` 的 `backend-security` 与 `frontend-security` 都红了。管理员要求「先看上游有没有已经修了，有就同步过来」。
- 结论：**上游都已修**，但上游一并升到了 Go 1.27.2，我们**只取依赖升级，不跟着升 Go 大版本**。
- 与生图教程无关：`main` 上它们本来就有，只是新披露/过期，被 10-05 之后的第一次扫描撞上。

## 为什么没有整库同步上游

`fork/main` 相对 `upstream/main` 领先 166 个提交、落后 2100 个（2026-10-10 实测）。AGENTS.md 坑 23 记着整库合并上游踩过的坑（47 个冲突）。
所以只比对**依赖文件**，把需要的几行升级拿过来。上游对应提交：`c61f6ebaa`（Go 1.27.2 + x/net，8 个文件）、`53158587a`（axios 1.20.0，2 个文件）。

## 后端：govulncheck 报了 10 个漏洞

- `golang.org/x/net@v0.56.0` → **v0.60.0**（GO-2026-6617、6612 等）。x/net v0.60.0 自己只要求 `go 1.26.0`，不需要 Go 1.27。
- Go 标准库 `net/http`、`net/textproto`、`crypto/tls`：**go1.26.6 → go1.26.9**（同一 1.26 系列的补丁版本）。
- 改动：`backend/go.mod`（`go 1.26.9`，x/net 带动 crypto v0.57.0、sys v0.48.0、text v0.42.0、term/sync/mod/tools 小幅上移）与 `go.sum`；
  Go 版本钉死的 8 处同步改：`backend-ci.yml`（2 处）、`release.yml`、`security-scan.yml` 里 `grep -q 'go1.26.x'`，以及 `Dockerfile`、`backend/Dockerfile`、`deploy/Dockerfile` 的 `golang:1.26.x-alpine`。
  **这会改生产镜像的构建器**；下次升 Go 要把这 8 处一起改，漏一处 CI 的「Verify Go version」就会红。
- 验证：`go build ./...`、`go vet`（repository / server）通过；本地 `govulncheck ./...` 复扫结果 `Your code is affected by 0 vulnerabilities`。

## 前端：pnpm audit 的高危

- `axios ^1.18.0` → **^1.20.0**（锁 1.20.0），消掉 7 个高危 GHSA（ReDoS、原型污染、HTTP/2 绕过等）。生成的 diff 与上游 `53158587a` 完全同形（`package.json` 1 行、锁文件 10 行）。
- `vue ^3.4.0 → ^3.5.43`（锁 3.5.26 → 3.5.43）：`@vue/server-renderer` 有高危 GHSA-g2v6-rqmx-r4w6（修复 ≥3.5.42）。Vue 3.5 系列内的补丁升级，上游同样是 3.5.43。
- `pnpm.overrides` 加 `"source-map-js@<1.2.2": ">=1.2.2"`：`@vue/compiler-core` 依赖链上的 source-map-js 默认仍解析到 1.2.1（GHSA-68fv-2mgg-jv7q，修复 ≥1.2.2）。上游同样用 override。
- `.github/audit-exceptions.yml`：两条 `xlsx` 例外在 2026-10-06 过期，续到 **2027-01-06**。采用上游的措辞，但先核实了我们代码里确实成立：
  `xlsx` 只在 `src/views/admin/UsageView.vue` 一处动态导入，只用 `aoa_to_sheet` / `book_new` / `XLSX.write` 写文件，没有 `XLSX.read` / `sheet_to_*` 之类的解析路径。
  npm 上没有 xlsx 的修复版本（`patched_versions` 为 `<0.0.0`），所以只能靠例外；到期前要么换写入库要么改用 SheetJS CDN 构建。

## 没有做、但上游做了的

- 上游还加了 `"dompurify@<3.4.14": ">=3.4.14"` override。我们的审计里 dompurify 有一批**中低危**（`--audit-level=high` 门禁不拦），没有夹带进这次，需要时单独做。
- 上游的 `nanoid` 例外到期日是 2026-10-13（比我们的 2026-12-06 更早），没有采用我们更晚的日期不动。

## 验证

见 PR 描述。需要特别记住的两点：
- **`frontend/audit.json` 是仓库里已跟踪的文件**，别在 `pnpm audit --json > audit.json` 之后 `rm`：2026-10-10 误删过一次。验证时把审计输出写到临时目录再喂给 `tools/check_pnpm_audit_exceptions.py`。
- 本机 Go 是 1.25.7，`GOTOOLCHAIN=auto` 会按 `go.mod` 自动下载 1.26.9，不用手装。

## 回滚

revert 本次合并提交即可（依赖和 Go 版本钉死的 8 处都在同一个提交里）。回滚后 Security Scan 会重新变红，但不影响构建和部署。
