# 独立公开页恢复模块

恢复基线：`81025b8e210e3ddb33a8e78a8aa02aba0c3abc80`（2026-10-09）。范围依据 `../docs/kedaya-fusion-scope.md` 的“登录、注册、首页主体样式保持原样”约定。本模块提供 `/`、`/home`、`/login` 的旧 Vue UI，控制台与其他公开路由继续由当前前端承接。

## 构建与校验

复用现有 `frontend/node_modules`，无需安装依赖或修改主项目配置：

```sh
cd frontend
node_modules/.bin/vite build --config public-pages/vite.config.ts
node_modules/.bin/vue-tsc --noEmit --project public-pages/tsconfig.json
# 等价构建入口，可从任意工作目录调用：
node public-pages/build.mjs
```

输出为 `public-pages/dist/index.html`、`dist/assets/*` 和 `dist/.vite/manifest.json`。Vite `base: './'`；HTML 的脚本与样式、CSS 中字体资源都使用相对 URL。组装时应保持 `dist` 内部目录结构，可整体复制到 `/integration/native/` 或带摘要目录，并从 HTML 或 Vite manifest 读取入口。页面路由 history base 固定 `/`，与资产目录无关。构建目标是 ES2020 浏览器资源，不含平台原生二进制；生产宿主的 `linux/arm64` 不改变这些资源。

模块由 `frontend/build.mjs` 编译后接入 assembler，不修改 Kedaya base 或生产设置。`dist/` 是可重新生成的产物，不提交到 Git。

## 来源文件清单

下列 57 个文件从基线 Git 对象逐个恢复。`sources.json` 逐文件记录来源路径、基线 commit 和原始字节 SHA-256；除下文列出的 LoginView 与 TotpLoginModal 适配，其余 55 个文件与基线一致。路径以本模块为根；`src/` 下各文件对应基线 `frontend/src/` 相同相对路径，`tailwind.config.js` 对应基线 `frontend/tailwind.config.js`。

```text
src/views/HomeView.vue
src/views/auth/LoginView.vue
src/components/layout/AuthLayout.vue
src/components/icons/Icon.vue
src/components/auth/DingTalkOAuthSection.vue
src/components/auth/EmailOAuthButtons.vue
src/components/auth/GitHubMark.vue
src/components/auth/GoogleMark.vue
src/components/auth/LinuxDoOAuthSection.vue
src/components/auth/LoginAgreementPrompt.vue
src/components/auth/OidcOAuthSection.vue
src/components/auth/TotpLoginModal.vue
src/components/auth/WechatOAuthSection.vue
src/components/CaptchaChallenge.vue
src/components/AliyunCaptchaWidget.vue
src/components/TencentCaptchaGate.vue
src/components/TurnstileWidget.vue
src/components/common/LocaleSwitcher.vue
src/components/common/ThemeToggle.vue
src/components/common/Toast.vue
src/composables/useHomepageStatus.ts
src/stores/app.ts
src/stores/auth.ts
src/api/auth.ts
src/api/client.ts
src/api/passkey.ts
src/api/status.ts
src/api/tokenRefresh.ts
src/api/url.ts
src/api/adminUIRequest.ts
src/api/admin/system.ts
src/utils/apiError.ts
src/utils/featureFlags.ts
src/utils/homepageStatus.ts
src/utils/oauthAffiliate.ts
src/utils/tencentCaptcha.ts
src/utils/theme.ts
src/utils/url.ts
src/style.css
src/styles/public-pages.css
src/types/index.ts
src/types/openaiReferrals.ts
src/types/payment.ts
src/api/admin/channelMonitor.ts
src/api/admin/channels.ts
src/api/admin/users.ts
src/constants/channel.ts
src/constants/channelMonitor.ts
src/constants/platformCatalog.ts
src/constants/platformCatalog.builtin.json
src/i18n/locales/en/common.ts
src/i18n/locales/en/landing.ts
src/i18n/locales/en/dashboard.ts
src/i18n/locales/zh/common.ts
src/i18n/locales/zh/landing.ts
src/i18n/locales/zh/dashboard.ts
tailwind.config.js
```

`types/index.ts` 的类型依赖闭包会触及 channel/channelMonitor/platformCatalog/users 等文件；保留这些文件以支持完整 TypeScript 校验，不通过整站 API barrel 扩大运行时依赖。

## 必要适配

- 新建 `src/main.ts`、`src/App.vue`、`src/router.ts`、`index.html` 和独立构建/类型配置。App 仅渲染路由与旧 Toast，并随路由、站点名称、语言更新标题；保留主题初始化、注入公开配置与 auth.checkAuth()。不引入整站公告、订阅或管理员设置 hook。
- 新建精简 `src/stores/index.ts`、`src/api/index.ts`、`src/components/layout/index.ts`，只导出公开页使用的 app/auth、auth/passkey API 与 AuthLayout。
- 独立 i18n 仅静态合并两种语言的 common、landing、dashboard，保留 `sub2api_locale` 和浏览器语言默认选择；dashboard 内含 profile/TOTP 翻译。语言变化由 App 响应式更新标题，无整站 router/adminSettings 动态 import。
- `src/utils/redirect.ts` 提取基线 `frontend/src/views/auth/OAuthCallbackView.vue:242-249` 的 `sanitizeRedirectPath`，补充非字符串、控制字符和反斜杠拒绝。LoginView 的密码、Passkey、2FA 三条成功路径统一调用该函数，默认仍到 `/home`，有效站内 redirect 保留 query/hash。
- LoginView 的密码显示按钮补充本地化可读名称及 pressed 状态；TotpLoginModal 补充 dialog 角色、标题/说明关联与每位验证码输入名称，不改变样式和业务逻辑。
- 路由仅三个路径渲染旧页面；`LegalDocument`（`/legal/:documentId`）及 Register、ForgotPassword、Dashboard、AdminDashboard 是可 resolve 的桥接名称，其余路径同样桥接。桥接使用 `window.location.assign(to.fullPath)` 交还当前前端；最终宿主必须按路径选择对应入口。已登录用户访问 `/login` 按角色跳仪表盘，`backend_mode_enabled` 非管理员例外保留；后台模式的匿名首页访问也按旧规则转登录页。
- Vite 本地内联 Tailwind/PostCSS，使用基线 Tailwind 主题，仅扫描本模块。禁用主项目 `.env` 自动加载，将 `VITE_API_BASE_URL` 固定为同源 `/api/v1`。旧 API 请求体、认证存储键 `auth_token`/`auth_user`/`refresh_token`/`token_expires_at`、刷新机制、OAuth referral、验证码及协议状态全部保留。
- 保留 Home 的 `home_content` > `compact_home_enabled` > 原统计首页优先顺序，不修改后端开关。保留所有密码登录、Passkey、GitHub/Google/LinuxDo/钉钉/微信/OIDC、Turnstile/腾讯/阿里云验证码、TOTP、协议弹层/勾选分支。
- `publicDir: false`，Vue 模板的绝对资源 URL 不改为 import：`/logo.svg`、`/logo.png`、`/community-placeholder.jpg` 由最终应用根目录提供，现有 `frontend/public` 中均存在。字体从现有 Public Sans 包构建到相对 assets。

## 验证边界

本地 Vite 构建和严格 `vue-tsc` 检查可独立运行。宿主前端集成检查校验 55 个未适配源码的历史摘要、两套入口和资源路径。浏览器功能验收使用本地认证与统计接口，覆盖密码错误、TOTP、登录重定向、主题持久化、控制台 CSS 隔离、前进后退和 375px 页面；不以该验收代替真实 WebAuthn、第三方 OAuth 或验证码供应商联调。主题沿用旧 `theme` storage key、`html.dark`、`data-theme`、`sub2api-theme-change` 事件。
