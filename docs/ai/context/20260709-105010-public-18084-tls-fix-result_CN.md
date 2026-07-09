# 公网 18084 TLS 链路修复结果

> 2026-07-09 10:50 JST 完成。

## 问题背景

`/v1/responses` 和 `/v1/chat/completions` 请求全部 502。

## 根因分析

1. `cli-proxy` 上游（`cliproxy-local-openai`，account id=1）的 `base_url` 配置为 `http://host.docker.internal:8317/v1`
2. sub2api 代码在 `SECURITY_URL_ALLOWLIST_ENABLED=false` 时调用 `ValidateURLFormat(raw, false)`，拒绝所有 HTTP URL
3. 之前尝试改 HTTPS，但 cli-proxy 当时没有启用 TLS
4. 后续尝试改回 HTTP，但代码仍拒绝

## 修复步骤

### 1. 为 cli-proxy 生成 TLS 证书

```bash
mkdir -p /Users/wujianxiang/CodeSpace/CLIProxyAPI/certs
openssl req -x509 -newkey rsa:4096 \
  -keyout /Users/wujianxiang/CodeSpace/CLIProxyAPI/certs/tls.key \
  -out /Users/wujianxiang/CodeSpace/CLIProxyAPI/certs/tls.crt \
  -days 3650 -nodes \
  -subj "/CN=localhost/O=CLIProxy" \
  -addext "subjectAltName=DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1"
```

### 2. 启用 cli-proxy TLS

修改 `config.yaml`：
```yaml
tls:
  enable: true
  cert: "certs/tls.crt"
  key: "certs/tls.key"
```

重启 cli-proxy：
```bash
kill <pid> && cd /Users/wujianxiang/CodeSpace/CLIProxyAPI && nohup ./cli-proxy-api > /tmp/cli-proxy.log 2>&1 &
```

验证：`openssl s_client -connect localhost:8317` 应显示证书

### 3. 更新 sub2api DB base_url

```sql
UPDATE accounts SET
  credentials = jsonb_set(credentials, '{base_url}', '"https://host.docker.internal:8317/v1"'),
  updated_at = NOW()
WHERE id = 1;
```

### 4. 容器内导入自签名 CA

```bash
# 重建系统 CA bundle（追加 CLIProxy 证书）
docker exec sub2api-candidate sh -c '
  for f in /usr/share/ca-certificates/*/; do
    cat "$f" 2>/dev/null
  done > /tmp/system-ca.crt &&
  cat /tmp/system-ca.crt /tmp/cli-proxy-ca.crt > /etc/ssl/certs/ca-certificates.crt
'

# 其中 /tmp/cli-proxy-ca.crt 通过以下方式传入：
docker cp /Users/wujianxiang/CodeSpace/CLIProxyAPI/certs/tls.crt sub2api-candidate:/tmp/cli-proxy-ca.crt
```

### 5. 重启 sub2api

```bash
docker restart sub2api-candidate
```

## 验证结果

| 测试 | 结果 |
|------|------|
| `/v1/responses` (gpt-5.5) | ✅ 200 |
| `/v1/chat/completions` (gpt-5.5) | ✅ 200 |
| 公网 api.aaccx.pw | ✅ 200 |

## 待持久化问题

当前 CA 证书是手动追加到运行中容器的，**重启容器后会丢失**。需要：

1. **方案 A（推荐）**：将 CA 证书嵌入 Docker 镜像，在 Dockerfile 中追加到系统 CA bundle
2. **方案 B**：在 `docker-entrypoint.sh` 启动脚本中自动导入 CA

文件位置：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/certs/tls.crt`
