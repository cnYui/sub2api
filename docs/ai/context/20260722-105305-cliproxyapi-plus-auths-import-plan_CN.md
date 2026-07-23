# CLIProxyAPI Plus 凭证导入计划

## 目标

- 将用户提供的 `plus-auths-20260722.zip` 中的四个凭证导入本地 `cliproxyapi-local-dev`。
- 导入后让本地链路 `Sub2API 18080 -> CLIProxyAPI 8317 -> 上游账号` 有可用额度。

## 安全边界

- 不在日志、文档、提交或回复中输出完整 token、refresh token、cookie、账号私密字段。
- 导入前备份现有 CLIProxyAPI auths 目录。
- 只操作本地 CLIProxyAPI 容器/本地 auths 文件，不操作公网 Cloudflare、生产 DB、Nginx。
- 验证只看数量、健康状态和错误类别；真实模型 smoke 需要可用 Sub2API 用户 Key 时另行执行。

## 执行步骤

1. 定位 `cliproxyapi-local-dev` 的 auths 挂载目录。
2. 只列出 zip 文件名和数量，不展示内容。
3. 备份当前 auths 目录到本地备份目录。
4. 将 zip 内四个凭证解压/复制到 CLIProxyAPI auths 目录。
5. 重启 `cliproxyapi-local-dev`，确认 8317 healthy。
6. 查看 CPA 日志中 auth entries 数量是否从 0 变为 4。
7. 验证 Sub2API 容器内访问 `https://cliproxyapi:8317/v1/models` 能完成 TLS，未带 CPA Key 返回 401 仍为预期。

## 回滚

- 删除本次导入的 auth 文件。
- 从备份目录恢复导入前 auths。
- 重启 `cliproxyapi-local-dev`。
