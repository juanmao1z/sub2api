# Agent Note: 统一出站 URL 私网地址策略

Status: implemented

## Problem

HTTP 上游传输层此前只有在 `security.url_allowlist.enabled=true` 时才执行 DNS 解析后的 IP 校验。即使部署配置明确设置 `allow_private_hosts=false`，allowlist 关闭时仍可能让域名解析到回环、RFC1918、链路本地或未指定地址后继续连接。

审计同时确认：CSRF 不是当前系统的可利用漏洞。用户和管理员认证依赖 `Authorization`/API-key 请求头，浏览器 Cookie 只承载 OAuth 临时状态，不承载认证会话；跨站请求无法伪造这些认证头。因此没有新增通用 CSRF 中间件。

## Decision

出站连接层现在以 `allow_private_hosts` 作为 DNS 解析校验的独立开关：

- `http_upstream` 只要 `allow_private_hosts=false` 就在实际请求前执行 `ValidateResolvedIP`，不再把 allowlist 开关作为必要前置条件。
- 重定向检查沿用同一策略，避免首跳安全但重定向目标落入私网。
- 协议入口继续保留原有格式校验和 allowlist 行为，以兼容现有管理员自定义中继与本地集成测试；真正的目标地址安全边界由出站传输层执行。
- `allow_private_hosts=true` 继续保留，兼容明确需要访问内网中继或私有服务的管理员配置。

测试覆盖字面量私网 URL 校验、显式私网 opt-in、解析 IP 传输层拒绝，以及 allowlist 关闭时仍启用解析校验的组合。

本次继续收紧出站连接边界：`ResolvePublicIPs` 对全部 A/AAAA 记录 fail closed，`SafeDialContext` 只向已验证的公网 IP 建立实际 socket；`ValidateResolvedIP` 保持原 API 兼容但不再承担 socket 钉住职责。启用私网禁止时，HTTP client 和 http_upstream 每次请求都重新校验，不再使用成功缓存；无代理直连和 TLS fingerprint 直连使用安全拨号器，无法在本地固定目标的代理路径直接拒绝。`http_upstream` 的 `cfg=nil` 按默认禁止私网处理。

## Alternatives considered

### 仅把生产环境的 `SECURITY_URL_ALLOWLIST_ENABLED` 改为 true

这能收紧当前配置，但要求维护完整的上游、CRS、pricing 等主机白名单，可能破坏现有管理员配置；也不能修复 allowlist 关闭时传输层安全开关失效的代码缺口。因此保留为运维侧进一步收紧选项，而不是唯一修复。

### 在每个协议入口重复加入私网字面量校验

这能更早拒绝部分输入，但会重复实现策略，且无法替代真实连接前的 DNS rebinding 防护；还会破坏现有使用本地 `httptest` 或内网中继的兼容路径。因此把不可绕过的检查放在共享传输层。

### 全局禁止所有私网地址

这会破坏明确使用内网 AI 中继、CRS 或定价服务的合法部署。系统已有 `allow_private_hosts` 配置表达运营者选择，因此只在该选项为 false 时 fail closed。

### 为所有状态变更请求增加 CSRF token

认证不使用 Cookie 会话，强加 CSRF token 会扩大前端和 API 契约而不增加当前威胁模型下的有效防护。OAuth 临时 Cookie 仍使用 HttpOnly、Secure、SameSite=Lax，并不作为 API 认证凭据。

## Consequences

收益：即使 allowlist 关闭，`allow_private_hosts=false` 也会约束实际 DNS 连接，降低管理员配置错误导致的内网探测和 DNS rebinding 风险；现有显式私网 opt-in 行为保留。

代价：开启私网禁止后，解析到内网地址的上游连接会被拒绝；管理员必须显式开启 `allow_private_hosts` 或使用公开目标。当前生产环境仍观察到 `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`，所以部署后该项仍是配置风险，而不是已关闭的生产策略。

覆盖限制：REA CLI `6.4.0` 可用但 native provider 缺少 pwntools 配置；Ghidra 未安装或设置 `GHIDRA_INSTALL_DIR`，因此本次二进制检查限于 `readelf`/ELF 元数据和 REA 可用性诊断，未完成 Ghidra 反编译或漏洞规则扫描。

验证：`go test ./...`、`go vet ./...`、前端 `npm run lint:check && npm run build` 通过；WebSocket 回归子集通过。
