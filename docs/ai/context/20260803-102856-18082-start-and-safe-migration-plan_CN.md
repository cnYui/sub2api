# 18082 启动与渠道迁移记录

## 运行态事实

- `18086` 当前为 `sub2api-upstream-latest`，用户确认 GPT 凭证位于该实例。
- `18087` 当前为 `sub2api-openai-billing-inner`，用户确认模型渠道位于该实例。
- 当前仓库需要在 `18082` 启动。
- 已有开发实例占用固定容器名 `sub2api-dev`、`sub2api-postgres-dev` 和 `sub2api-redis-dev`，因此 18082 必须使用隔离的容器名、数据目录和 Compose 项目名。

## 安全边界

- 不读取、导出、展示或复制 API Key、OAuth token、密码、HMAC secret、加密凭证或数据库中的 `credentials` 内容。
- 可以迁移不含认证秘密的渠道元数据，例如渠道名称、启停状态、模型映射和功能开关。
- 认证秘密由用户在目标实例的管理界面或密钥管理系统中重新录入，并在迁移后单独验证。

## 实施决策

- 新增 `deploy/docker-compose.18082.yml`，只覆盖容器名、端口和本地数据目录。
- 启动使用 Compose 项目名 `sub2api-official-18082`，绑定 `127.0.0.1:18082`，不停止或重建现有 18080、18086、18087 实例。
- 启动所需的本地数据库密码由当前 PowerShell 进程临时生成，不写入仓库文件和回复。

## 回滚边界

- 仅删除 `sub2api-official-18082` Compose 项目及其新增的 18082 数据目录。
- 不删除或重建现有实例的 PostgreSQL、Redis 容器和 volume。
