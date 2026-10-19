# Agent Note: 渠道状态矩阵颜色映射修复

Status: implemented

## Problem

渠道监控将无分数状态输出为 `health-operational`、`health-degraded`、`health-failed` 和 `health-error`。固定 CSS 只为 `health-healthy`、`health-warning`、`health-critical` 等别名提供颜色，导致状态点和时间矩阵色块在这些真实状态下没有颜色。

## Decision

保持固定 JS 资源及其校验摘要不变，在 `frontend/kedaya/assemble.mjs` 对固定 CSS 使用唯一锚点补入作用域匹配的状态规则。正常及分数 7–10 显示绿色，延迟/降级及分数 4–6 显示黄色，不可用/错误及分数 0–3 显示红色；未知状态维持灰色。前端集成验证检查实际组装 CSS 中的语义状态和颜色。

## Alternatives considered

直接修改固定 CSS 可让规则更直接，但会改动固定输入资源并使其来源摘要失效，因此选择在受校验的组装步骤补丁。只添加无作用域的全局规则虽简单，却会扩大颜色规则影响范围，未采用。

## Consequences

状态类名与颜色语义重新一致，矩阵色块及状态点都能辨识。基础资源升级后，唯一锚点及两个组件作用域标识需要随新资源重新核对。未知或缺少数据仍显示中性灰色，不将未知伪装为失败。

## Testing

`frontend/kedaya/verify.mjs` 通过独立组装输出验证 operational、degraded、failed、error 和 unknown 的颜色规则；发布构建后须再核对产物摘要与生产状态。
