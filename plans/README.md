# Material Relay 前端动效计划

| 编号 | 标题 | 严重度 | 状态 | 覆盖审计项 |
| --- | --- | --- | --- | --- |
| 001 | 建立 Material Relay 视觉与动效 token | HIGH | TODO | 5、12 |
| 002 | 重构应用壳与高频导航动效 | HIGH | TODO | 3、4、9、10 |
| 003 | 统一 Origin-aware Overlay 与 Popover | HIGH | TODO | 1、7、8、12 |
| 004 | 让 Toast 与进度指标可中断且 GPU-only | HIGH | TODO | 2、11 |
| 005 | 清理持续运动并补齐 reduced-motion | HIGH | TODO | 5、6、9 |
| 006 | 增加稀有状态反馈并统一页面层材质 | MEDIUM | TODO | 8、10、全部 Gate 通过机会 |

## 推荐顺序

`001 -> 002 -> 003 -> 004 -> 005 -> 006`

001 提供所有后续计划使用的 token；002-005 可在 token 完成后按顺序执行并保持测试绿；006 最后进行页面级视觉整合和截图验收。

## 依赖

- 002-006 依赖 001。
- 006 依赖 002 的应用壳和 003 的 Overlay 契约。
- 最终完成标准以 `plans/006-purposeful-page-feedback.md` 的全量视觉验证为准。
