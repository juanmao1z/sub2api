# API 错误与网络排查

按网络、配置、错误码、服务容量和重试限制的顺序排查问题，通常可以更快定位 Codex 或其他 API 客户端的故障原因。

## 1. 排查网络和配置问题

### 第一步：测试密钥和模型

先在控制台使用当前 API 密钥和模型发起一次最小请求：

- 测试成功：优先检查本地客户端配置或网络链路。
- 测试失败：检查 API 密钥状态、所属分组、余额和模型权限。

### 第二步：排查网络

- 更换 VPN 节点后重试。
- 优先使用虚拟网卡（TUN）模式；系统代理模式可能导致长连接不稳定。
- `client_gone`、流式中断和偶发超时通常与网络或代理链路有关。

### 第三步：排查客户端配置

- 确认 API 密钥创建在正确分组，并且该分组支持请求模型。
- 在 `codex++` 中新增供应商配置，确认 `base_url` 使用 `https://api.zhouz.online/v1`。
- 确认 `base_url` 位于对应的 provider 配置块中，而不是写在其他配置层级。
- 启用配置后无法对话时，先退出 Codex 并重新打开；仍然失败时，备份 `~/.codex/config.toml`，再校验 TOML 格式并优先合并或修复目标 provider。
- 只有在备份完成、确认会话数据不受影响时，才让工具重新生成配置文件。
- 仍无法使用时，检查 `codex++` 版本，更新到最新版或回退到稳定版本。

测试请求成功但正式使用异常，可能来自本地配置、分组或模型权限、余额、网络或上游状态。请根据下面的错误信息继续定位。

## 2. 遇到报错时的排查清单

按以下顺序逐项尝试：

1. 开启虚拟网卡（TUN）模式，避免仅使用系统代理模式。
2. 更换 VPN 节点。
3. 退出当前会话并新建会话；需要保留上下文时，可以复制会话 ID 后继续。
4. 完全退出 Codex、`codex++` 或 IDE，再重新打开。
5. 先备份 `config.toml`，再校验并合并或新建 provider。
6. 重启电脑，排除系统代理或网络栈的临时状态。

## 3. 常见错误码和流式报错

### `401 Unauthorized`

- 检查响应 body 和 request ID，确认是 API 密钥无效、被禁用、Base URL 错误，还是鉴权头没有生效。
- 在控制台确认密钥状态、分组、余额，再核对客户端实际使用的配置。
- 需要重建时先备份 `~/.codex/config.toml`，不要把删除配置作为固定解决方案。

### `stream disconnected before completion`

如果同时看到 `stream closed before response.completed` 或 `idle timeout waiting for SSE`，优先检查网络节点、VPN 和代理对 SSE 长连接的支持。

### `client_gone`

表示请求链路在客户端一侧中断，优先判断网络、代理或客户端是否被关闭，不要先判断为账号失效。

### `Reconnecting... 1/5` 反复重连

先切换 VPN 节点并重启客户端。如果持续出现，检查代理配置、TUN 模式和本地防火墙是否允许长连接。

### `503 No available channel for model ... under group ...`

当前 API 密钥分组不支持请求模型，或该分组暂时没有可用渠道。请在模型列表中确认支持该模型的分组，再切换模型或分组。

## 4. 服务繁忙或容量提示

出现以下提示时，可能是模型上游、当前渠道、分组容量或本地网络暂时不可用：

```text
We're currently experiencing high demand, which may cause temporary errors.
```

记录发生时间、模型、分组、request ID 和响应 body。先重试，或切换到已确认支持的模型和分组，再判断是否需要切换网络。

## 5. `429 Too Many Requests`

例如：

```text
exceeded retry limit, last status: 429 Too Many Requests
```

`429` 表示某一层触发了限流或额度约束，可能与 API 密钥频率、分组容量、上游配额、账户余额或活动状态有关。请依次检查响应 body、控制台余额或套餐、模型支持分组和服务公告，再决定是否切换分组。

## 6. 提交排查信息

仍无法解决时，请提供发生时间、请求模型、分组、错误码、request ID 和脱敏后的响应 body。不要提交完整 API 密钥、完整配置文件或包含个人信息的日志。

参考：[API 错误与网络排查](https://doc.ergouzi.life/help/api-errors)
