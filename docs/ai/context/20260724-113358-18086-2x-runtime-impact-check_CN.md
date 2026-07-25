# 18086 调整到 2x 的运行态影响核对

时间：2026-07-24

## 本次范围

- 只核对运行态。
- 不修改 `18086` 配置。
- 不重启容器。
- 不触碰数据库。

## 当前容器事实

当前相关容器：

- `sub2api-dev`：`127.0.0.1:18080 -> 8080`，健康。
- `sub2api-upstream-latest`：`127.0.0.1:18086 -> 8080`，健康。
- `cliproxyapi-local-dev`：`127.0.0.1:8317 -> 8317`，健康但不参与当前调度。
- `sub2api-public-nginx-local`：`127.0.0.1:8080 -> 8080`。

## 当前转发事实

本机 `sub2api-public-nginx-local` 配置：

```nginx
upstream sub2api_local_18080 {
    server host.docker.internal:18080;
}

proxy_pass http://sub2api_local_18080;
```

因此当前本机 nginx 入口 `127.0.0.1:8080` 指向 `18080`，不是直接指向 `18086`。

## 当前 CPA 状态

外层 `sub2api-dev` 数据库中：

| 账号 | base_url | status | schedulable |
|---|---|---|---|
| `cliproxy-local-openai` | `https://cliproxyapi:8317/v1` | active | false |
| `sub2api-latest-openai-upstream` | `http://host.docker.internal:18086/v1` | active | true |

结论：

- CPA 容器虽然在运行，但外层账号 `cliproxy-local-openai` 不可调度。
- 当前可调度 OpenAI 上游是 `sub2api-latest-openai-upstream`，指向 `18086`。
- 当前模型请求链路是 `Sub2API 外层 -> 18086 内层 latest Sub2API -> OpenAI OAuth 账号池`。

## 当前倍率配置事实

`18080` 外层配置：

```yaml
billing:
    unit_price_multiplier: 1.8
```

`18086` 内层配置当前没有 `billing` 段：

```yaml
default:
    rate_multiplier: 1
```

这说明：

- 当前 `1.8x` 基础单价倍率明确存在于 `18080` 外层。
- `18086` 内层目前没有配置 `billing.unit_price_multiplier`，按默认 `1.0x` 运行。

## 如果要修改 18086 到 2x

需要在 `sub2api-upstream-latest` 挂载目录：

```text
D:\CodeWorkSpace\sub2api-upstream-latest\deploy\upstream_data\config.yaml
```

新增：

```yaml
billing:
    unit_price_multiplier: 2.0
```

然后重启 `sub2api-upstream-latest`。

原因：

- 该配置由后端启动时读取。
- 当前代码没有确认存在运行时热加载配置。
- 仅修改文件但不重启，不能保证进程内配置生效。

## 是否影响公网服务

取决于公网实际入口：

1. 若公网仍是 `公网 -> 18080 -> 18086`
   - 重启 `18086` 时，`18080` 自身健康接口和控制台可能仍可用。
   - 但 OpenAI/Codex 模型请求会受影响，因为唯一可调度上游指向 `18086`。
   - 影响表现可能是短暂 `502/503/504` 或账号池不可用。

2. 若公网已经直接指向 `18086`
   - 重启 `18086` 会直接影响公网 API 与控制台。
   - 影响范围更大。

本机证据显示 `sub2api-public-nginx-local` 指向 `18080`；但公网 Cloudflare/Nginx 的真实指向仍需在执行前再次确认。

## 是否还需要 CPA

当前不需要。

原因：

- CPA 对应账号已设为 `schedulable=false`。
- 当前可调度链路已经是 Sub2API 内层 latest。
- 修改 `18086` 不需要启动或修改 CPA。

## 建议执行方式

如果后续要正式改：

1. 先确认公网入口真实指向。
2. 备份 `18086` 的 `config.yaml`。
3. 修改 `18086` 配置新增 `billing.unit_price_multiplier: 2.0`。
4. 重启 `sub2api-upstream-latest`。
5. 验证：
   - `http://127.0.0.1:18086/health`
   - 外层 `http://127.0.0.1:18080/health`
   - 公网 `/v1/models`
   - 公网一条低成本 `/v1/responses`
   - 外层 `usage_logs` 与 `usage_facts` 是否 settled

## 关键结论

- 是，改 `18086` 配置后大概率需要重启 `sub2api-upstream-latest` 才会生效。
- 是，如果公网模型请求当前经过 `18086`，重启会造成短暂公网模型请求不可用。
- CPA 当前不参与调度，不是这次修改对象。
- 若只想改变用户实际扣费，当前事实显示 `18080` 外层已经有 `billing.unit_price_multiplier=1.8`；是否还要改 `18086`，取决于你是否希望内层也按 `2.0x` 自己计费/展示。
