# gpt-image-2 生图 timeout 改为 300 秒操作说明

## 结论

这次要改的不是 Sub2API，也不是 CLIProxyAPI，而是调用方生图 provider 的 Python 服务。

日志里的模块名是：

```text
ylcraft.openai_sdk_image
```

因此优先修改文件应为该 Python 服务项目中的：

```text
ylcraft/openai_sdk_image.py
```

当前 `/Users/wujianxiang/CodeSpace/sub2api` 仓库里没有这个文件；它应该在部署 `provider=aaccx-gpt-image-2` 的调用方服务仓库或线上运行环境里。

## 如何定位文件

进入生图 provider 的 Python 项目根目录后执行：

```bash
find . -name 'openai_sdk_image.py'
rg -n "AsyncOpenAI|OpenAI\\(|images.generate|OpenAISDK-Image|timeout" .
rg -n "aaccx-gpt-image-2|ylcraft.openai_sdk_image" .
```

如果服务运行在容器里，可进入对应容器后查：

```bash
docker exec -it <生图服务容器名> sh
find /app -name 'openai_sdk_image.py'
rg -n "AsyncOpenAI|images.generate|timeout" /app
```

## 推荐修改方式

在 `ylcraft/openai_sdk_image.py` 里找 OpenAI SDK 客户端初始化，通常类似：

```python
from openai import AsyncOpenAI

client = AsyncOpenAI(
    api_key=api_key,
    base_url=base_url,
)
```

改为：

```python
from openai import AsyncOpenAI

client = AsyncOpenAI(
    api_key=api_key,
    base_url=base_url,
    timeout=300.0,
)
```

如果代码是每次调用前临时创建 client，也同样在创建处加 `timeout=300.0`。

如果 client 是全局复用、不方便改初始化处，可以在生图调用前加请求级配置：

```python
client = client.with_options(timeout=300.0)
response = await client.images.generate(
    model="gpt-image-2",
    prompt=prompt,
    n=1,
    size="1024x1024",
)
```

如果项目已有 provider 配置，例如：

```yaml
providers:
  aaccx-gpt-image-2:
    timeout: 120
```

则直接改为：

```yaml
providers:
  aaccx-gpt-image-2:
    timeout: 300
```

配置型改法优先于硬编码改法，因为后续可以不用重新改源码。

## 不要改的位置

不要优先修改以下位置：

- `sub2api/deploy/docker-compose.yml` 里的 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT`：当前是 `900`，它是图片流数据间隔超时，不是这次约 120 秒总等待超时。
- `sub2api/deploy/config.example.yaml` 里的 `gateway.image_stream_data_interval_timeout`：同上。
- `CLIProxyAPI/internal/runtime/executor/codex_openai_images.go`：该路径使用 `NewProxyAwareHTTPClient(..., 0)`，`0` 表示 CLIProxyAPI 没有设置 HTTP client 总超时。
- `GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT`：当前默认为 `0`，不是这次约 120 秒取消来源。

## 验证方法

修改后重启调用方生图服务，再发起同类 `gpt-image-2` 请求。

重点看两点：

1. 请求不再固定在约 `120-125s` 失败。
2. 如果上游生图耗时超过 120 秒，调用方日志不应再出现由本地等待超时导致的 `context canceled`。

建议观察日志关键字：

```bash
rg -n "OpenAISDK-Image|images.generate|context canceled|timeout|elapsed|duration" <日志文件或日志目录>
```

如果改成 300 秒后仍在约 120 秒失败，说明还有第二层超时，下一步再查调用方外层任务队列、HTTP 网关、反向代理或平台 provider wrapper 的 timeout 配置。
