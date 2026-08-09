# Sub2API - zhouz.online 定制版

<div align="center">
  <img src="frontend/public/logo.png" alt="zhouz.online" width="88" />

  <p><strong>基于 Sub2API 的个人生产定制版本</strong></p>

  [线上站点](https://api.zhouz.online/home) ·
  [官方项目](https://github.com/Wei-Shaw/sub2api) ·
  [定制升级流程](docs/custom-upgrade-workflow.md)
</div>

> [!IMPORTANT]
> 本仓库不是官方 Sub2API 的同步镜像。`main` 代表 zhouz.online 当前可部署的完整定制版本，官方代码通过 `upstream` 远端和 release tag 按需合并。

## 项目定位

本项目基于 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 开发，当前定制基线为官方 `v0.1.173`。官方项目负责通用的账号池、API Key、计费、并发控制、负载均衡、请求转发、管理后台以及 PostgreSQL/Redis 基础能力；本仓库在此基础上增加 zhouz.online 的品牌、公开状态页、外部充值入口、排行榜接入和生产部署流程。

本 README 重点记录相对于官方 `v0.1.173` 的增量。官方通用功能、配置项和基础安装说明请以[官方仓库](https://github.com/Wei-Shaw/sub2api)为准。本次官方升级包含渠道监控 v2、Grok 音频与搜索能力、注册邮箱域名额度策略、Gemini 图片计费修复，以及多项调度、计费和稳定性改进。

## 相对官方的主要差异

| 领域 | 本仓库增加的内容 | 主要实现位置 |
| --- | --- | --- |
| 品牌 | zhouz.online 青绿色渐变 `Z` 图标、站点图标缓存刷新、Public Sans 字体 | `frontend/public/logo.png`、`frontend/index.html` |
| 公开首页 | 重新设计的轻量首页、运行时长、近 1 小时活跃用户、当日成功率、累计 Token | `frontend/src/views/HomeView.vue` |
| 公开状态 API | 首页聚合状态与实时 API 并发查询，包含短时缓存、并发请求合并和超时保护 | `backend/internal/service/ops_homepage_status.go`、`ops_public_status.go` |
| 登录与社群 | 登录成功默认进入 `/home`，首页社群按钮与本地图片弹窗 | `frontend/src/views/auth/`、`frontend/src/views/HomeView.vue` |
| 外部充值 | 登录后 `/recharge` 页面、内嵌支付页、新窗口打开入口、支付域名 CSP 放行 | `frontend/src/views/user/ExternalRechargeView.vue` |
| 自定义菜单 | 公开菜单可见性规范化、用户/管理员菜单隔离、移除账号菜单中的官方 GitHub 入口、内嵌页面兼容 | `backend/internal/service/setting_public.go`、`AppHeader.vue`、`CustomPageView.vue` |
| 排行榜接入 | 首页快捷入口和 `usage-leaderboard` 自定义页面适配 | `frontend/src/views/HomeView.vue`、`CustomPageView.vue` |
| 生产构建 | GitHub Actions 完成前后端与 Docker 镜像构建并发布到 GHCR，服务器只拉取和运行镜像 | `.github/workflows/release.yml` |
| 升级发布 | 上游合并检查、定制项保护、单应用容器切换、健康检查和回滚记录 | `docs/custom-upgrade-workflow.md` |

## 1. 品牌化公开首页

公开首页已针对 zhouz.online 重写，不再沿用官方默认落地页。

- 使用白底青绿色渐变 `Z` 作为站点 Logo 和 favicon。
- 首页导航、标题、状态信息和快捷入口采用紧凑的生产状态页布局。
- 展示站点运行时长、近 1 小时活跃用户、当日成功率和累计处理 Token。
- 快捷入口直接连接控制台、兑换/充值页面和使用排行榜。
- 登录与 OAuth 回调没有显式跳转目标时默认返回 `/home`。
- 首页社群按钮打开带微信群二维码的无障碍弹窗，支持遮罩、关闭按钮和 Escape 关闭。
- 支持中英文界面、深色模式和移动端导航。

首页状态来自匿名只读接口：

```text
GET /api/v1/status/homepage
GET /api/v1/status/concurrency
```

后端对首页聚合结果使用分钟级缓存，对实时并发结果使用秒级缓存，并通过 singleflight 合并同时到达的刷新请求，避免公开页面对数据库和 Redis 形成额外压力。

## 2. 外部充值入口

本仓库增加了登录后可访问的 `/recharge` 路由，用于在控制台内打开指定支付页面。

- 路由保留 `requiresAuth` 登录保护。
- 支付页默认内嵌在应用布局中。
- 右上角提供在新窗口打开的备用入口。
- CSP 明确允许 `frame-src https://pay.ldxp.cn`。
- 相关路由、iframe 地址和安全头均有回归测试。

该页面是额外的外部支付入口，不替代官方 Sub2API 自带的支付和订单能力。

## 3. 自定义菜单与排行榜

后台设置中的 `custom_menu_items` 会进入公开设置和动态路由。本仓库在官方能力之上补充了以下约束：

- 只向匿名或普通用户公开允许其查看的菜单项。
- 管理员专用菜单不会通过公开设置泄露。
- 自定义页面继续支持外部 URL 和 Markdown 页面。
- `usage-leaderboard` 使用站内嵌入模式，并隐藏重复的“新窗口打开”操作。
- 首页提供排行榜快捷入口。

排行榜服务由独立的 `sub2api-leaderboard` sidecar 提供，不包含在本仓库中。升级主应用时需要继续验证以下兼容契约：

- `GET /api/v1/auth/me`
- `public.usage_logs` 与 `public.users`
- Compose 服务名 `sub2api`、`postgres` 和网络 `sub2api-network`
- `/custom/usage-leaderboard` 与 `/leaderboard/` 的反向代理规则

## 4. 面向小型服务器的生产构建

官方 Dockerfile 会在镜像构建阶段编译前后端。为了降低生产服务器的内存和网络压力，本仓库增加 `Dockerfile.prebuilt-binary`：

1. 在本机完成前端构建，输出到 Go embed 目录。
2. 在本机构建 Linux amd64 后端二进制。
3. 在本机使用 `Dockerfile.prebuilt-binary` 构建完整运行镜像；Docker Hub 不可用时，用 `Dockerfile.rebase-binary` 从上一版已验证镜像离线重建应用层。
4. 在本机导出、压缩并计算镜像归档 SHA-256。
5. 服务器只校验归档、执行 `docker load` 并切换应用容器。
6. 通过镜像内 `sub2api -version` 校验版本和构建元数据。

生产服务器不执行 `go build`、`pnpm build` 或 `docker build`。生产环境只替换 `sub2api` 应用容器，不重建 PostgreSQL、Redis 或排行榜服务。Compose 覆盖文件会在切换前备份，新容器未通过健康检查时可恢复上一镜像。

完整命令、归档结构和回滚步骤见[定制升级流程](docs/custom-upgrade-workflow.md)。

## 5. 上游升级策略

本仓库的 `main` 是生产定制分支，不应直接用官方 `main` 覆盖。推荐流程：

1. 从 `upstream` 获取目标 release tag。
2. 在临时升级分支合并新的官方 tag。
3. 重点检查充值路由、CSP、自定义菜单、首页状态 API 和嵌入式前端。
4. 运行前端、后端和 embed 回归测试。
5. 生成新镜像并仅切换应用容器。
6. 完成线上健康检查后，将验证过的提交快进到本仓库 `main`。

每次升级必须保护这些定制边界：

```text
/recharge
https://pay.ldxp.cn
custom_menu_items
/api/v1/status/homepage
/api/v1/status/concurrency
usage-leaderboard
backend/internal/web/dist
```

## 本地验证

前端：

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

& pnpm --dir frontend run test:run
if ($LASTEXITCODE -ne 0) { throw "frontend tests failed" }
& pnpm --dir frontend run build
if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
```

后端定制区域：

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

& go -C backend test -tags unit ./internal/service -run 'TestSettingService_GetPublicSettings_|TestOpsService_GetPublic'
if ($LASTEXITCODE -ne 0) { throw "backend service tests failed" }
& go -C backend test -tags embed ./internal/web
if ($LASTEXITCODE -ne 0) { throw "backend embed tests failed" }
```

## 目录说明

| 路径 | 用途 |
| --- | --- |
| `frontend/` | Vue 3 前端和 zhouz.online 定制页面 |
| `backend/` | Go API、公开状态聚合和嵌入式前端 |
| `deploy/` | 官方基础部署文件 |
| `Dockerfile.prebuilt-binary` | 预编译二进制运行镜像 |
| `Dockerfile.rebase-binary` | 无法访问镜像仓库时，从上一版可信运行镜像离线重建 |
| `CUSTOM_UI_NOTES.md` | 当前定制边界和生产约束 |
| `docs/custom-upgrade-workflow.md` | 上游升级、构建、部署和回滚流程 |

## 风险与责任

本项目涉及第三方 AI 服务、账号池、API 转发和外部支付页面。部署者必须自行确认上游服务条款、当地法律法规、数据保护要求和支付合规要求，并独立承担账号、资金、数据和服务中断风险。

本仓库的定制部署、运营、收费和用户管理行为与官方 Sub2API 作者及贡献者无关。

## 致谢与许可证

感谢 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 及其贡献者提供基础项目。本仓库继续遵循原项目许可证，详情见 [LICENSE](LICENSE)。
