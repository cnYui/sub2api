# compress-agents-context Skill 设计

## 目标

创建个人 Codex skill：`compress-agents-context`。之后用户调用该 skill 时，Codex 能自动压缩当前项目的 `AGENTS.md`，并把精简后的长期上下文保存到 `docs/ai/context/YYYYMMDD-HHMMSS-agents-memory-condensed_CN.md`。

## 使用场景

- 用户要求压缩、精简、整理或归档 `AGENTS.md`。
- 用户要求把项目长期记忆压缩到 `docs/ai/context/`。
- `AGENTS.md` 运行态流水账过长，需要保留当前状态、风险约束和追溯索引。

## 设计取舍

推荐方案是 `SKILL.md + scripts/validate_agents_context.py`：

- `SKILL.md` 负责语义压缩流程，要求读取项目规则、先写 plan、再生成精简版。
- 验证脚本负责确定性检查，包括命名、目录、敏感模式和文件存在性。
- 不把压缩本身做成纯脚本，因为“什么是长期有效、什么只是流水账”需要语义判断。

## 压缩原则

- 保留：协作规则、架构定论、当前运行态、高风险操作、支付/计费/SMTP/部署等仍影响后续行为的信息。
- 折叠：逐日执行细节、重复健康检查、已由历史文档承载的完整过程。
- 追溯：关键历史只保留文档路径索引。
- 禁止：覆盖历史文档、写入密钥、直接替换根目录 `AGENTS.md`，除非用户明确要求。

## 验证策略

- 初始化 skill 后运行官方 `quick_validate.py`。
- 运行 `validate_agents_context.py` 检查本轮生成的上下文文档。
- 执行 `git diff --check` 与 `git status --short`。
- 不创建子代理压力测试：当前工具说明要求用户未明确请求子代理时不要 spawn；本轮以刚刚完成的手工压缩过程作为 RED 样本。
