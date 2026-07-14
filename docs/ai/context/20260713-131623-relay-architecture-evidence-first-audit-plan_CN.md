# 中转站架构证据优先审计实施计划

> **供执行代理使用：** 本计划只允许只读取证。并行子任务必须返回脱敏证据，不得编辑配置、重启服务、写数据库、做生产压测或发起高费用模型请求。

**目标：** 形成一份基于当前运行态证据的《中转站架构优化与安全加固白皮书》。

**架构：** 按边缘入口、Nginx/sub2api、CPA/上游、宿主机与依赖四个证据域并行取证，再由主审计统一交叉验证。所有结论采用“已证实/高概率推断/待验证”三级标签。

**技术栈：** Cloudflare Tunnel、Homebrew Nginx、Docker Desktop、Go/sub2api、CLIProxyAPI、PostgreSQL、Redis、TLS、SSE/OpenAI Responses API。

---

### 任务 1：建立变更与证据边界

- [ ] 记录 `git status --short --branch`，保护已有未提交文件。
- [ ] 读取项目 `AGENTS.md`、最新压缩记忆及与 Tunnel、Nginx、TLS、流式、并发相关历史文档。
- [ ] 给每条历史结论建立“当前重新核验”条目，历史文档不得直接计为当前证据。

### 任务 2：核验边缘入口和源站暴露

- [ ] 用 `dig`/`curl -I`/TLS 握手记录公开 DNS、边缘协议和响应头，不发送 Authorization。
- [ ] 用 `pgrep`、`ps`、`lsof -nP -iTCP -sTCP:LISTEN` 核验 `cloudflared`、Nginx 和宿主机公网监听。
- [ ] 读取 `cloudflared` 进程参数与实际配置的非敏感字段，确认 ingress 主机名、origin URL、连接数和协议。
- [ ] 读取 macOS 防火墙、pf 与路由可见状态；不能证明云路由器/NAT 状态时标为待验证。

### 任务 3：核验 Nginx 有效配置和流式行为

- [ ] 运行 `nginx -T` 并提取 `listen`、real IP、proxy protocol、buffering、cache、timeout、keepalive、limit、日志格式和 header 规则。
- [ ] 核验 `$request_time`、`$upstream_connect_time`、`$upstream_header_time`、`$upstream_response_time` 是否实际记录。
- [ ] 只对健康或无费用端点做 `curl -N`/HTTP 版本探测；模型流式行为若需付费则只读源码和历史日志。

### 任务 4：核验 sub2api 边界、连接池和计费一致性

- [ ] 定位运行容器、端口映射、网络模式、readiness/healthcheck、资源限制和副本数。
- [ ] 从运行库或管理接口只读取上游 `base_url`、协议、账号池模式和并发数，所有凭据字段仅输出“存在/缺失”。
- [ ] 追踪 HTTP Transport、请求取消、SSE Flush、重试、错误分类、usage 幂等和终态处理源码。
- [ ] 核验日志是否可能输出 Authorization、Cookie、请求正文、encrypted reasoning 或完整 Key。

### 任务 5：核验 CPA 入口、上游路由和敏感数据

- [ ] 核验 CPA 进程、版本、监听、TLS、进程托管、配置权限和凭据存储类型，不读取完整凭据值。
- [ ] 提取入站全局/每 Key/端点限流，确认 sub2api 到 CPA 是否只使用共享内部 Key。
- [ ] 追踪连接池、上游账号选择、429/5xx/网络错误重试、首字节前后重试边界、账号亲和性和 thinking signature 处理。
- [ ] 核验日志正文、Header、OAuth 凭据和 Core Dump 风险。

### 任务 6：核验容量、依赖和单点

- [ ] 记录 CPU/内存、Docker Desktop 限额、文件描述符、临时端口范围、TCP 状态、连接数和进程线程/句柄。
- [ ] 读取 PostgreSQL/Redis 连接池和持久化、健康检查、备份、实例数及容器重启策略。
- [ ] 用当前并发上限和可获得的平均流时长应用 Little's Law；缺少分位数据时给出参数化公式和采集方法。

### 任务 7：核验 Cloudflare 官方能力

- [ ] 仅引用 Cloudflare 官方文档，记录查询日期、产品套餐适用范围和页面 URL。
- [ ] 核验 Tunnel origin 协议、连接冗余、HTTP/2/HTTP/3、SSE/长请求、代理超时、真实客户端 IP、Rate Limiting/WAF/Access 的当前能力和限制。
- [ ] 无法从本地确认控制面规则是否已启用时标为待验证，不通过公开行为反推完整 WAF 配置。

### 任务 8：综合分析与白皮书

- [ ] 为每项风险填写证据、P0-P3、触发条件、影响、修复、成本、回滚和验收。
- [ ] 生成当前与目标拓扑、协议表、超时预算表、限流矩阵、SLI/SLO 和告警阈值。
- [ ] 给出 Nginx 最小差异、单机加固方案、跨主机 HA 方案、分阶段路线图和故障注入矩阵。
- [ ] 检查全文不存在完整密钥、Cookie、Token、数据库密码、私钥、请求正文或未经证据支持的确定性表述。
