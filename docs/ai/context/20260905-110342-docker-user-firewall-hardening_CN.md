# DOCKER-USER 防火墙兜底：阻断公网入站容器

## 背景：Docker 会绕过 UFW

Docker 直接往 iptables 的 `DOCKER` 链插 DNAT 规则，优先级在 UFW 之前。因此 compose 里任何 `0.0.0.0:PORT:PORT` 形式的端口发布，**即使 UFW 没有放行该端口，公网照样能连上**。这是 Docker 部署最经典的安全陷阱。

`DOCKER-USER` 是 Docker 官方预留的、唯一能压过其自身规则的链（在 `FORWARD` 链第 1 位被调用）。

## 核查结论：改造前本就没有暴露，但留着一个雷

### 当时实际是安全的

| 证据 | 内容 |
|---|---|
| 端口发布情况 | `sub2api-redis 6379/tcp`、`sub2api-postgres 5432/tcp` —— **仅 EXPOSE，未发布**；`sub2api 127.0.0.1:8080->8080/tcp` |
| iptables DOCKER 链 | 只有一条 DNAT，且 `destination 127.0.0.1`，公网包匹配不上 |
| 宿主机监听 | 只有 `*:22` |
| 公网实测 | 8080 / 5432 / 6379 全部不可达；22 可连接（控制组，证明测试有效） |

`6379/tcp` 与 `0.0.0.0:6379->6379/tcp` 是完全不同的两件事：前者只声明容器内网可见，后者才会插 DNAT 绕过 UFW。compose 里 postgres 和 redis 的 `ports` 是注释掉的，只留了「如需调试可临时添加 `127.0.0.1:5433:5432`」的提示。

### 但存在一个结构性隐患

基础 `deploy/docker-compose.yml` 第 29 行：

```yaml
- "${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080"
```

而 `.env` 里写着 `BIND_HOST=0.0.0.0`。

**只要有人漏掉 `-f docker-compose.vps.yml`，直接 `docker compose -f docker-compose.yml up -d`，8080 就会绑到 `0.0.0.0` 并绕过 UFW 暴露到公网。** 当时唯一挡住它的是 override 文件里的 `ports: !override` —— 属于「记得带上正确参数」型防护，不是结构性防护。

## 环境事实（决定规则设计）

| 项目 | 值 |
|---|---|
| 公网接口 | `eth0`（`${OPS_VPS_HOST}`，**同时有全局 IPv6 地址与默认路由**） |
| docker 网桥 | `br-0c387e63bbb8`（`172.18.0.1/16`） |
| `FORWARD` 链默认策略 | `DROP`，`DOCKER-USER` 位于第 1 位 |
| `DOCKER-USER` 链 | 改造前为空 |
| userland-proxy | 启用（2 个 `docker-proxy` 进程） |

**IPv6 已启用，因此 `ip6tables` 必须同步配置**，否则等于留了个 v6 后门。

## 规则设计

```
1) -i eth0 -m conntrack --ctstate RELATED,ESTABLISHED -j RETURN
2) -i eth0 -j DROP
```

### 第 1 条绝不能少

容器访问上游 API 时，**返回包也是从 eth0 进入并走 `FORWARD` 的**。只写 DROP 会把所有上游调用的返回流量一起掐断，服务立刻不可用。这是本次改造最大的风险点。

### 各类流量的走向

| 流量 | 匹配情况 | 结果 |
|---|---|---|
| 容器 → 互联网（上游 API 调用） | `-i br-xxx`，不匹配 `-i eth0` | 放行 |
| 互联网 → 容器（上述调用的返回包） | `-i eth0` + `ESTABLISHED` | 命中规则 1，RETURN |
| 互联网 → 容器（主动入站） | `-i eth0` + `NEW` | 命中规则 2，**DROP** |
| 宿主机 → 容器（cloudflared → `127.0.0.1:8080`） | 走 `OUTPUT`，不经 `FORWARD` | 不受影响 |
| 容器 ↔ 容器（app ↔ postgres/redis） | `-i br-xxx -o br-xxx` | 放行 |

本架构中没有任何容器需要公网入站 —— 用户流量全部由 cloudflared **主动出站**建立，应用端口只绑 `127.0.0.1`。因此全量拦截公网入站是安全的。

## 变更过程的安全措施

1. **全量备份** v4/v6 规则到 `/opt/sub2api/fw-backup-<ts>/`，并建 `fw-backup-latest` 软链
2. **自动回滚保险**：`systemd-run --on-active=5min --unit=fw-rollback`，脚本检查 `/opt/sub2api/fw-confirmed` 标记，不存在则自动 `iptables-restore` 还原。**验证不通过就自动恢复，无需人工干预**
3. 应用规则 → 立即验证 → 通过后才写确认标记、停掉回滚定时器

## 持久化

- 脚本 `/usr/local/sbin/docker-user-firewall.sh` —— 幂等（先删后插），v4/v6 各自检测链是否存在后再施加
- 单元 `docker-user-firewall.service` —— `After/Requires/PartOf=docker.service`，已 `enable`。`PartOf` 使 docker 重启时本单元同步重启，重新施加规则
- 手动重新施加：`systemctl restart docker-user-firewall`

**已实测**：清空 `DOCKER-USER` 链（v4/v6 各归零）→ `systemctl restart docker-user-firewall` → 两条规则完整恢复；脚本连跑两次后仍是 2 条规则，幂等性成立。

## 验证结果

| 检查项 | 结果 |
|---|---|
| 三域名 `/health` | 全部 200 |
| 真实 API 调用（穿透上游） | HTTP 200，返回 `"still ok"` |
| 容器出站 | 退出码 0，`cdn-cgi/trace` 返回出口 IP `${OPS_VPS_HOST}` |
| 容器间通信 | postgres `ok` / redis `PONG` |
| 端口 22（控制组） | 可连接 |
| 端口 8080 / 5432 / 6379 | 不可达 |
| `RETURN` 计数 | 86 包（返回流量在放行） |
| `DROP` 计数 | 0（无公网主动入站尝试） |

`DROP` 计数持续为 0 是正常的，说明确实没有公网在敲门；`RETURN` 计数会随容器访问上游而增长。

## 配套改动：`BIND_HOST` 收紧

`.env` 的 `BIND_HOST` 由 `0.0.0.0` 改为 `127.0.0.1`。效果：

| 场景 | 改前 | 改后 |
|---|---|---|
| **漏掉 override**（要防的情况） | `host_ip: 0.0.0.0` | `host_ip: 127.0.0.1` |
| 正常带 override | `host_ip: 127.0.0.1` | `host_ip: 127.0.0.1` |

override 本来就赢，因此当下解析结果与运行状态均不变，**无需重启容器**。`.env` 已留时间戳备份。

至此该风险有三层防护：

1. `.env` 的 `BIND_HOST=127.0.0.1` —— 默认不往公网绑
2. `docker-compose.vps.yml` 的 `ports: !override` —— 显式再锁一次
3. `DOCKER-USER` 链的 DROP 规则 —— 即使前两层被绕过（如手动 `docker run -p 0.0.0.0:6379:6379`），公网入站仍进不来

前两层是「配置正确」，第三层是「结构性阻断」。

## 两次测试翻车（判定方法的教训）

### 1. `nc -z` 在 Git Bash 下不工作

初次公网端口测试中，**四个端口全部报「不可达」，包括当时正在用于 SSH 的 22 端口**。若不设控制组，会把这个假阴性当成「端口都关着」的决定性证据。

改用 PowerShell `Test-NetConnection` 后 22 正确显示可连接。

**教训：测端口可达性必须带一个已知可达的控制组。**

### 2. `wget` 遇 HTTP 401 退出码非 0，被误判为「网络不通」

用 `wget https://api.openai.com/v1/models`（无认证）测容器出站，返回 401 导致 wget 退出码为 1，显示「不通」。实际网络完全正常。

更早一次显示「✓ 通」同样不可信 —— 命令是 `wget ... | head -c 80 >/dev/null 2>&1 && echo 通`，管道的退出码取自 `head`，**永远是 0，那个测试从头到尾没检查过任何东西**。

**教训：测容器出站要用返回 200 的无认证端点（如 `https://www.cloudflare.com/cdn-cgi/trace`），或直接以真实 API 调用作为地面真相。注意管道会掩盖前序命令的退出码。**

## 回滚方法

```bash
# 停用并移除持久化
systemctl disable --now docker-user-firewall.service
# 还原到改造前的规则
BK=$(readlink -f /opt/sub2api/fw-backup-latest)
iptables-restore  < "$BK/rules.v4"
ip6tables-restore < "$BK/rules.v6"
```

`.env` 的 `BIND_HOST` 可从同目录 `.env.bak.<ts>` 恢复。
