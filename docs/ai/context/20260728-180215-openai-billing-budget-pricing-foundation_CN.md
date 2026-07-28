# OpenAI 计费预算与价格快照基础

## 决策

- 单次 OpenAI 请求的 durable authorization 上限固定为 2 USD；不足时仅对未显式指定输出上限的文本请求减少输出 Token，最小输出为 256 Token。
- 显式输出上限、输入固定成本、最低输出无法放入单一资金来源时，请求前返回预算不足，不转发上游。
- 嵌入类输入可使用零输出预算，只 hold 固定输入成本。
- 预算记录保存价格来源 hash、模型、service tier、分组倍率和最终每 Token 单价；价格按 service tier 与分组倍率各应用一次。

## 当前仓库事实

实施计划提到的 `deploy/data/model_pricing.json` 不存在。当前生效价格资源为 `backend/resources/model-pricing/model_prices_and_context_window.json`，测试直接解析该文件并用 `test-pricing-hash` 验证快照。

## 边界

文本 Token 估算已覆盖 JSON 结构、角色、tools、函数名、参数和 schema；data URL / file_data 的 base64 原文不作为文本 Token。图片、PDF 的解析与独立预算仍由后续附件任务实现。

本阶段只执行本地 Go 单元测试，未访问远程价格源，未启动候选应用，也未改变公网服务。
