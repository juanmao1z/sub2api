# Agent Note: 渠道卡片历史时间条颜色修复

Status: implemented

## Problem

上一条 [渠道状态矩阵颜色映射](./2026-10-19-channel-status-colors.md) 修复的是 V2 矩阵状态点和色块，但用户截图展示的是 V1 卡片的 V3 历史时间条。该组件根据健康状态生成 `healthy`、`warning`、`critical`，之后又用可用率阈值类覆盖颜色；用户看到的卡片时间条因此没有按健康状态显示预期颜色。

## Decision

在 `frontend/kedaya/assemble.mjs` 为固定卡片时间条使用唯一组装补丁：保留 `healthy`、`warning`、`critical`、`unknown` 的语义 class，并对可见的组件作用域定义静态背景颜色。健康状态决定颜色，不再由可用率再次覆盖；未知状态使用中性灰色。集成测试验证运行时映射和实际组装 CSS。

## Alternatives considered

继续扩充可用率阈值对应的 Tailwind 类看似能恢复颜色，但会继续让成功率而不是健康状态决定色条语义，因此不采用。直接修改固定 CSS 会改变锁定资源；仍在组装层追加作用域颜色规则，以保留源资源校验。

## Consequences

卡片历史时间条现在按状态稳定呈现绿色、黄色、红色，未知数据呈灰色。后续基础资源更新时必须重新确认 timeline 的组件作用域标识和状态枚举。矩阵状态的独立修复仍由关联笔记记录。

## Testing

`frontend/kedaya/verify.mjs` 检查 V2 矩阵和 V3 卡片时间条的语义状态、颜色及状态映射不被可用率阈值覆盖。部署后核对线上固定发布 CSS 摘要及健康状态。
