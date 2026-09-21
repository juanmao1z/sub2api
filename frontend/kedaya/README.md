# Kedaya 界面适配

正常执行 `pnpm run build` 即生成本项目的原首页与 Kedaya 控制台，现有 Dockerfile 使用同一构建命令。

`src/views/HomeView.vue` 及其依赖保持原实现。构建首先运行原有 Vite 流程，然后由 `assemble.mjs` 合并已固定版本的 Kedaya 前端资源。`/home` 使用原首页；`/tickets`、`/admin/tickets` 和 `/custom/*` 使用本站完整的工单及自定义页面实现，并加载控制台主题；其余路由使用 Kedaya 入口。跨入口导航会重新加载文档。账号会话仍使用现有 localStorage 字段。

站点名称、Logo、功能开关、自定义菜单和接口地址来自当前后端注入的 `window.__APP_CONFIG__` 或 `/api/v1/settings/public`，业务请求仍发送到本站 `/api/v1`。

## 文件

- `vendor/`：2026-09-21 获取的 `https://kedaya.ai/` 已发布主站及无限画布构建文件，保留第三方许可证注释。
- `reference-manifest.json`：下载来源和 SHA-256。每次构建校验使用到的上游资源。
- `runtime/`：入口选择、样式隔离、跨入口导航，以及用户和管理员共用的顶栏对齐样式。
- `assemble.mjs`：合并构建产物并执行三项精确站点适配。
- `verify.mjs`：构建后资源完整性和导航检查。

构建产物位于 `backend/internal/web/dist`。`integration/build-manifest.json` 记录首页原始哈希与适配文件哈希。HTML、运行时模块和 Kedaya 资源使用按内容生成的发布目录，避免浏览器与 CDN 的旧缓存混用不同版本。

## 接口差异

1. 自定义 API 端点：读取本项目已有的公开配置 `custom_endpoints`。
2. 站点 Logo：使用当前配置的 URL 或 data URI；为空时沿用本站 `/logo.png`。
3. 控制台菜单：采用本站选定的功能入口，包含用户和管理员工单。使用说明沿用本站四栏内置文档；排行榜沿用本站的嵌入显示规则。

模型广场是否显示遵循当前站点开关。当前线上配置关闭模型广场，所以正式适配不会自行开启它。

## 检查

在 frontend 目录构建后执行 `node --test kedaya/verify.mjs`。Go 静态资源检查在 backend 目录执行 `go test -tags embed ./internal/web`。

GitHub CI 会执行正式前端构建、资源校验和 Go 嵌入静态文件检查。`vendor/` 已显式纳入版本管理，完整克隆仓库即可复现构建。

此集成固定的是上游已发布构建版本，无法获得其未公开的 Vue/TypeScript 源工程。上游升级时，应重新下载并核对接口适配锚点；锚点或校验不匹配会使构建失败，避免静默混用版本。已发布资源的原许可和品牌权利仍归相应权利人所有。
