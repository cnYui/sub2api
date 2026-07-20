# macOS 服务器 Docker 部署指南：CLIProxyAPI + Sub2API

这份指南面向一台 macOS 服务器，同时部署 `CLIProxyAPI-private` 和 `sub2api`，并通过独立的 Docker bridge 把两个容器连起来。

目标拓扑：

```text
sub2api-network
  ├─ sub2api
  ├─ postgres
  └─ redis

sub2api-cliproxy-local
  ├─ sub2api
  └─ cliproxyapi-local-dev
```

约束：

- PostgreSQL / Redis 只留在 `sub2api-network`
- `cliproxyapi-local-dev` 只加入 `sub2api-cliproxy-local`
- `cliproxyapi-local-dev` 只绑定本机回环 `127.0.0.1:8317`
- 证书私钥、`auths/`、`.env` 不要提交到 Git

本指南涉及的启动文件：

- CLIProxyAPI-private：`Dockerfile`、`docker-compose.sub2api-local.yml`、`docker-build.sh`、`.env.example`、`config.example.yaml`
- Sub2API：`deploy/docker-compose.local.yml`、`deploy/docker-compose.cliproxy-local.yml`、`deploy/.env.example`

## 1. 目录准备

建议把两个仓库放在同一父目录下：

```bash
mkdir -p ~/apps
cd ~/apps
git clone https://github.com/cnYui/sub2api.git
git clone https://github.com/cnYui/CLIProxyAPI-private.git
```

后续命令默认在下面两个目录中执行：

- `~/apps/sub2api`
- `~/apps/CLIProxyAPI-private`

## 2. 前置条件

- 安装 Docker Desktop
- 安装 `openssl`

检查版本：

```bash
docker version
docker compose version
openssl version
```

## 3. 创建共享网络

先创建外部 bridge 网络：

```bash
docker network inspect sub2api-cliproxy-local >/dev/null 2>&1 || docker network create sub2api-cliproxy-local
```

如果你想用别的名字，也可以改成：

```bash
export SUB2API_CLIPROXY_NETWORK=your-network-name
```

但两边仓库必须保持一致。

## 4. 准备 CLIProxyAPI-private

进入仓库并准备本地文件：

```bash
cd ~/apps/CLIProxyAPI-private
cp .env.example .env
mkdir -p auths logs certs/authority certs/runtime
cp config.example.yaml config.yaml
```

### 4.1 生成本地 CA 和叶子证书

`generate-sub2api-local-tls.ps1` 是 Windows / PowerShell 侧的辅助脚本。  
在 macOS 服务器上，直接用 `openssl` 生成同样的文件：

如果 `certs/authority/ca.key` 或 `certs/runtime/tls.key` 已经存在，先备份再轮换，不要直接覆盖。

```bash
openssl genrsa -out certs/authority/ca.key 3072
openssl req -x509 -new -sha256 -days 3650 \
  -key certs/authority/ca.key \
  -out certs/runtime/ca.crt \
  -subj "/CN=Sub2API Local Development CA/O=Sub2API"

openssl genrsa -out certs/runtime/tls.key 3072
openssl req -new \
  -key certs/runtime/tls.key \
  -out certs/runtime/tls.csr \
  -subj "/CN=cliproxyapi/O=CLIProxyAPI"

cat > certs/runtime/tls.ext <<'EOF'
authorityKeyIdentifier=keyid,issuer
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:cliproxyapi,DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1
EOF

openssl x509 -req \
  -in certs/runtime/tls.csr \
  -CA certs/runtime/ca.crt \
  -CAkey certs/authority/ca.key \
  -CAcreateserial \
  -out certs/runtime/tls.crt \
  -days 825 \
  -sha256 \
  -extfile certs/runtime/tls.ext

openssl verify -CAfile certs/runtime/ca.crt certs/runtime/tls.crt
rm -f certs/runtime/tls.csr certs/runtime/tls.ext certs/authority/ca.srl
chmod 600 certs/authority/ca.key certs/runtime/tls.key
```

### 4.2 配置 `config.yaml`

至少确认这几项：

```yaml
host: "0.0.0.0"
port: 8317

tls:
  enable: true
  cert: "/CLIProxyAPI/certs/tls.crt"
  key: "/CLIProxyAPI/certs/tls.key"

auth-dir: "/root/.cli-proxy-api"
```

`api-keys` 里填你在 Sub2API 里配置好的上游账号，不要把真实密钥写进 Git。

### 4.3 配置 `.env`

建议至少补这几个变量：

```bash
SUB2API_CLIPROXY_NETWORK=sub2api-cliproxy-local
CLI_PROXY_PORT=8317
CLI_PROXY_TLS_DIR=./certs/runtime
CLI_PROXY_CONFIG_PATH=./config.yaml
CLI_PROXY_AUTH_PATH=./auths
CLI_PROXY_LOG_PATH=./logs
USAGE_EVENTS_ENABLED=true
YUI_USAGE_EVENT_URL=http://sub2api:8080/api/internal/usage-events
YUI_USAGE_EVENT_TOKEN=<same-as-sub2api-local-usage-event-token>
YUI_USAGE_EVENT_HMAC_SECRET=<same-as-sub2api-local-usage-event-hmac-secret>
```

如果你准备从现有宿主机直接导入认证文件，把 `auths/` 放到这个目录即可。

## 5. 启动 CLIProxyAPI 容器

构建并启动：

```bash
docker compose -f docker-compose.sub2api-local.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.sub2api-local.yml ps
docker compose -f docker-compose.sub2api-local.yml logs -f
```

如果要更新镜像：

```bash
docker compose -f docker-compose.sub2api-local.yml build --no-cache
docker compose -f docker-compose.sub2api-local.yml up -d
```

## 6. 准备 Sub2API

进入 Sub2API 仓库：

```bash
cd ~/apps/sub2api/deploy
cp .env.example .env
```

至少确认这些值：

```bash
SUB2API_CLIPROXY_NETWORK=sub2api-cliproxy-local
LOCAL_USAGE_EVENT_TOKEN=<same-as-CLIProxyAPI-YUI_USAGE_EVENT_TOKEN>
LOCAL_USAGE_EVENT_HMAC_SECRET=<same-as-CLIProxyAPI-YUI_USAGE_EVENT_HMAC_SECRET>
CLIPROXY_CA_CERT_PATH=../../CLIProxyAPI-private/certs/runtime/ca.crt
```

然后启动 Sub2API 和它自己的数据网络：

```bash
docker compose -f docker-compose.local.yml -f docker-compose.cliproxy-local.yml up -d
```

这里的 `docker-compose.local.yml` 负责 `sub2api + postgres + redis`，  
`docker-compose.cliproxy-local.yml` 只把 `sub2api` 接到共享 bridge。

## 7. 启动顺序

推荐顺序：

1. 创建 `sub2api-cliproxy-local`
2. 启动 `CLIProxyAPI-private`
3. 启动 `sub2api`
4. 在 Sub2API 管理界面把 `cliproxy-local-openai` 的 `base_url` 改成 `https://cliproxyapi:8317/v1`

如果你还在做首次导入账号，优先把 `auths/` 和 `config.yaml` 备好，再启动 CLIProxyAPI。

## 8. 验证

宿主机验证：

```bash
curl --cacert ~/apps/CLIProxyAPI-private/certs/runtime/ca.crt https://127.0.0.1:8317/health
curl --cacert ~/apps/CLIProxyAPI-private/certs/runtime/ca.crt \
  -H "Authorization: Bearer <cliproxyapi-api-key>" \
  https://127.0.0.1:8317/v1/models
```

容器内验证：

```bash
docker exec sub2api wget -qO- https://cliproxyapi:8317/health
docker exec sub2api wget \
  --header="Authorization: Bearer <cliproxyapi-api-key>" \
  -qO- https://cliproxyapi:8317/v1/models
```

共享网络检查：

```bash
docker network inspect sub2api-cliproxy-local
```

## 9. 常用回滚

停止共享桥接版 CLIProxyAPI：

```bash
cd ~/apps/CLIProxyAPI-private
docker compose -f docker-compose.sub2api-local.yml down
```

如果要把 Sub2API 的上游临时切回宿主机端口，改回：

```text
https://host.docker.internal:8317/v1
```

## 10. 提醒

- 不要把 `certs/authority/ca.key` 提交到 Git
- 不要把 `auths/`、`.env`、真实 API Key 提交到 Git
- `sub2api-network` 和 `sub2api-cliproxy-local` 不是一回事，不要混用
