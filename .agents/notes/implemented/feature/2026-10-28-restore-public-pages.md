# Agent Note: 恢复切换 Kedaya 前的登录页与首页

Status: implemented

## Problem

固定 Kedaya 编译基座接管全部路由后，/login 与 /home 的布局和样式随控制台一同变化。用户要求恢复整体切换前的公开页面。历史融合范围原本明确保留登录和首页；覆盖配色不能还原旧结构，恢复整套旧前端又会替换现有控制台。

## Decision

`frontend/public-pages/` 复用 `81025b8e210e3ddb33a8e78a8aa02aba0c3abc80` 的原生 Vue 首页与登录页。其 HomeView、LoginView、AuthLayout 和公开页 CSS 与整体切换前最后版本 `7fe6a45076d74672f2d3cdf7a77e85ce5f895934` 相同。模块的 sources.json 记录 57 个历史文件及原始 SHA256；55 个文件逐字节保留，LoginView 仅增加安全重定向及密码按钮语义，TotpLoginModal 仅补充对话框与验证码输入名称，不改变原布局或样式。

根地址、/home、/login（含末尾斜线）选择 native 入口，其他路由保留 review 入口。`frontend/build.mjs` 先用 Vite 编译公开页，再调用现有组装器。公开页使用相对资源 base，脚本、CSS 和字体复制到带摘要的 integration 目录；资源摘要、原始源码修订和入口描述一并写入 build-manifest.json。固定 Kedaya 基座及原始摘要不修改。开发预览和正式 Docker 构建调用相同入口，生成的 public-pages/dist 不提交。

两套入口共享同源 auth_token、auth_user、refresh_token、token_expires_at、语言和主题设置。跨入口的 history push/replace、站内链接和浏览器 popstate 完整导航，避免全局 CSS 与挂载状态混合；同入口保持 Vue Router 导航。query、fragment 和 replace 语义保留。原生路由能 resolve LegalDocument 等必要名称，再交还当前控制台。当前控制台的登录防护、退出和支付流程继续使用现有实现。

公开页保留旧密码、TOTP、Passkey、OAuth、验证码、协议与站点设置分支。默认登录成功到 /home，有有效站内 redirect 时保留目标；非字符串、外站、协议相对地址、控制字符和反斜杠不作为跳转目标。首页保留 home_content > compact_home_enabled > 统计首页的原优先级，不修改生产设置。独立入口避免引入整站公告和管理员初始化，精简 barrel 与 i18n 只承接公开页依赖。

## Related decisions

检索活跃笔记没有已有公开页恢复决定。[画布与指南](./2026-10-19-canvas-guides-v0217.md)、[渠道矩阵颜色](../bug-fix/2026-10-19-channel-status-colors.md) 和 [时间条颜色](../bug-fix/2026-10-19-channel-card-timeline-colors.md) 继续持有控制台功能，不被本次修改替代。[v0.2.19 发布适配](../process/2026-10-28-v0219-arm64-release.md) 继续持有版本、后端和发布约束；本篇部分取代其公开页沿用固定基座的范围。来源比对也核对 frontend/docs/kedaya-fusion-scope.md 与历史融合计划中保留公开页的约定。

## Alternatives considered

仅给当前公开页加 CSS 覆盖可以减少产物和路由改动，但不能精确恢复旧布局与交互，因此复用历史源码。恢复整套旧 Vue 前端可以直接恢复旧页面，但会替换现有控制台、画布、指南及已有适配，因此只恢复公开入口。把两套应用挂在同一个文档可减少整页跳转，但会混合全局样式和会话内状态，因此在入口边界重载文档。

直接复用旧编译快照可以减少维护源码，但历史归档只包含原生源码与 Kedaya 编译资源，没有原定制首页和登录页的可用编译快照，因此保留必要源码与独立构建配置。首次引入 Kedaya 之前的 9 月版本也有旧 UI，但旧公开页后来一直保留并更新过主题；整体替换前的 10 月版本与既有保留约定一致，因此采用最后保留的页面版本。

## Testing

- 独立 Vite 构建与严格 vue-tsc 检查通过；资源清单验证 55 个未适配源码摘要、入口、懒加载、CSS/字体路径、原控制台补丁、画布和渠道颜色。前端集成、导航与画布测试共 21/21 通过。
- `go test -count=1 -tags='unit embed' ./internal/web` 通过，验证嵌入前端处理。
- 本地浏览器使用线上公开设置及临时认证/统计接口，覆盖真实页面代码：旧首页统计、社群弹窗与 Escape、主题持久化、密码显示、密码错误不建立会话、TOTP 完成前无令牌、成功后跳转指定控制台、控制台不加载 native CSS、query/fragment 保留、浏览器后退、找回密码桥接、根地址别名。
- 普通密码登录及拒绝外站 redirect 的浏览器检查通过。1440px 桌面、375px 手机与 812px 横屏无横向溢出；明暗主题截图已保存，测试启用 reduced-motion。完整流程没有未捕获运行时错误，排除有意测试的 401 后没有应用 console 错误。
- 浏览器不提交生产登录、OAuth、真实支付或模型请求。第三方 OAuth/验证码 SDK 与真实 WebAuthn 的供应商联调未执行；对应旧代码分支完整保留。

## Consequences

公开页可以独立维护，恢复原统计首页与紧凑认证布局，同时继续使用当前控制台。代价是增加必要源码与一个编译入口，跨入口导航需要重新加载文档；来源清单、摘要检查、类型检查和浏览器边界测试使这项隔离可复核。

本次不修改后端、数据库迁移、认证存储契约或生产站点设置。正式发布仍构建 linux/arm64、绑定清洁提交与发布标签，备份数据库并验证恢复，只切换 sub2api；线上核验入口、资源摘要、健康状态和保留服务 ID。发布结果及回滚信息写入对应 deploy-artifacts/<release>/release.json。
