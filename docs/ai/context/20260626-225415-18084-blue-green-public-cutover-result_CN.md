# 18084 蓝绿环境切公网结果

## 执行时间

- 时间：2026-06-26 22:54 JST
- 执行范围：按用户要求跳过 Task 1 公网备份，从 Task 2 开始执行。

## 切换动作

- 修改 `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- 修改 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- 将两个配置中所有 Sub2API 反代从 `http://127.0.0.1:18080` 改为 `http://127.0.0.1:18084`
- 执行 `nginx -t` 通过
- 执行 `nginx -s reload` 成功

## 当前公网入口

- `api.aaccx.pw` 已经通过 Nginx 反代到 `127.0.0.1:18084`
- `aaccx.pw` 中归 Sub2API 的路由也已通过 Nginx 反代到 `127.0.0.1:18084`
- yui.web/shop 的 `4173` 路由未修改

## 候选运行态

- App：`sub2api-candidate`
- 镜像：`sub2api-candidate:20260626-220602-payment-template-30e66c82580f`
- 端口：`127.0.0.1:18084->8080`
- 状态：healthy
- DB：`sub2api-candidate-postgres`，healthy
- Redis：`sub2api-candidate-redis`，healthy

## 命令行验证

- `curl http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`
- `curl https://api.aaccx.pw/health` 返回 `{"status":"ok"}`
- `curl https://api.aaccx.pw/v1/models` 携带无效测试 Key 返回应用层 `401 INVALID_API_KEY`，说明公网 `/v1/*` 已到达 Sub2API
- `nginx -t` 输出 `syntax is ok` 和 `test is successful`
- Nginx 配置中 Sub2API 反代均为 `127.0.0.1:18084`
- 公网前端资源：
  - `assets/app-index-CXmPznNo.js`
  - `assets/index-nffSQZgD.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-misc-DB0Q8XAf.css`
  - `assets/pkg-vue-BqGtxt06.js`

## 浏览器真实验证

### 普通用户

- 账号：`2799523972@qq.com`
- 公网登录成功，进入 dashboard
- 左侧存在“购买订阅”
- 访问 `https://aaccx.pw/purchase` 成功
- 购买页显示：
  - 29 元订阅池
  - 39 元订阅池
  - 59 元订阅池
  - 99 元订阅池
  - GPT 流量包 5 刀
  - GPT 流量包 10 刀
  - GPT 流量包 20 刀
  - 1% 手续费文案
- 点击 99 元订阅池进入确认区，显示支付宝、手续费和 `确认支付 ¥99.99`
- 点击 GPT 流量包 5 刀进入确认区，显示支付宝、手续费和 `确认支付 ¥2.02`
- 未点击最终确认支付按钮，因此未创建新的支付订单
- 页面未出现“支付功能暂未开放”“充值功能暂未开放”“确认支付功能未开放”

### 管理员

- 账号：`xiaobianfuai@gmail.com`
- 公网登录成功，进入 dashboard
- 管理员导航显示：
  - 用户管理
  - 分组管理
  - 渠道管理
  - 订阅管理
  - 账号管理
  - 订单管理
  - 系统设置
- 点击用户管理成功打开 `/admin/users`，显示候选库最新用户列表，包含 ID 47 等记录
- 点击订单管理菜单成功展开：
  - 支付概览
  - 订单管理
  - 订阅套餐
- 点击支付概览成功打开 `/admin/orders/dashboard`，显示今日收入、总收入、支付方式分布、支付宝统计和消费排行
- 点击订单管理成功打开 `/admin/orders`，显示支付宝订单、手续费、订单状态和查看按钮
- 点击普通购买入口成功打开 `/purchase`
- 点击 29 元订阅池进入确认区，显示支付宝、手续费和 `确认支付 ¥29.29`
- 未点击最终确认支付按钮，因此未创建新的支付订单
- 管理端验证未出现 401、无权限或权限不足错误

## 结论

公网已经切到 18084 候选环境。健康检查、API 入口、普通用户购买页、管理员管理页和管理员购买页均已通过验证。

## 后续建议

- 观察公网 15-30 分钟，重点看 API 401/502、登录、购买页、支付回调、订单列表。
- 稳定后安排一次标准化固化：不要长期依赖 `sub2api-candidate` 容器名和 candidate worktree 路径，建议把候选 app/DB 状态固化回标准公网栈。
