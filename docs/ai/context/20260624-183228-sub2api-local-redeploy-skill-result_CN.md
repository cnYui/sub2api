# Sub2API 本地重部署 Skill 创建结果

## 结果

- 已创建本地 Codex skill：
  `/Users/wujianxiang/.codex/skills/sub2api-local-redeploy`
- Skill 名称：
  `sub2api-local-redeploy`
- Skill 校验结果：
  `Skill is valid!`

## 用途

该 skill 用于在用户明确要求时，将本地 `Sub2API` 最新主分支源码构建为 `weishaw/sub2api:latest`，并只重建 `sub2api` 容器。

典型触发语：

- “执行 Sub2API 本地重部署 skill”
- “跑镜像替换脚本”
- “本地重启 Sub2API”
- “更新公网到本地最新版本”
- “重新部署 sub2api”

## 固定行为

- 自动寻找当前签出 `main` 的 worktree 作为 Docker build context。
- 使用 Docker Desktop CLI：
  `/Applications/Docker.app/Contents/Resources/bin/docker`
- 使用项目脚本：
  `/Users/wujianxiang/CodeSpace/sub2api/deploy/redeploy-sub2api-image.sh`
- 使用 Compose 文件：
  `/Users/wujianxiang/CodeSpace/sub2api/deploy/docker-compose.local.yml`
- 使用 Compose env：
  `/Users/wujianxiang/CodeSpace/sub2api/deploy/.env.scheme-a.local`
- 部署后验证本地与公网健康检查，并确认公网前端资源 hash。

## 注意

- 该 skill 不复制部署脚本，始终复用项目中的 `deploy/redeploy-sub2api-image.sh`。
- 不打印、记录或提交 `deploy/.env.scheme-a.local` 的内容。
- 该操作会短暂影响 `https://api.aaccx.pw/v1/*`，正在进行的 Codex 流式请求可能断开。
