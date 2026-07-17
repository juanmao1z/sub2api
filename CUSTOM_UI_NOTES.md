# 自定义 UI 与部署约束

这个目录作为本机 Sub2API 定制版主工作区使用。以后生成部署包、预编译二进制和临时交付物时，放在本目录下的 `deploy-artifacts/` 或 `build-local/`，不要再散放到 `D:\Desktop`。

## 仓库约束

- 个人 fork 远端：`https://github.com/juanmao1z/sub2api.git`
- 官方上游远端：`https://github.com/Wei-Shaw/sub2api.git`
- 建议远端命名：`origin` 指向个人 fork，`upstream` 指向官方仓库。
- 不要直接在官方仓库分支上改定制功能；每次升级都从官方 tag 或官方分支派生新的定制分支。

## 当前线上定制

- 线上部署路径：`/opt/sub2api-deploy`
- 镜像暂存路径：`/opt/sub2api-images`
- compose 文件：`docker-compose.local.yml` + `docker-compose.override.yml`
- 当前升级目标镜像：`sub2api-custom:0.1.160-ui1`
- 当前回滚镜像：`sub2api-custom:0.1.156-ui5`
- 当前定制基线：官方 `v0.1.160` + 本仓库自定义品牌首页、默认 `/home` 登录跳转、社群弹窗、账号菜单移除官方 GitHub 入口、`/recharge` 页面、`usage-leaderboard` 自定义菜单入口与首页实时并发状态
- 线上切换方式：只修改 `/opt/sub2api-deploy/docker-compose.override.yml` 中 `sub2api` 服务镜像，再执行 `docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d sub2api`

## 自定义充值页

- 路由：`/recharge`
- 访问要求：登录后可见，路由必须保留 `requiresAuth: true`
- 支付地址：`https://pay.ldxp.cn/shop/1WGCPCG0`
- 页面形式：在应用内嵌入 iframe，中间显示支付网站，右上角提供新窗口打开按钮。
- CSP 要求：后端默认 CSP 和安全头增强逻辑必须允许 `frame-src https://pay.ldxp.cn`

如果浏览器里 iframe 仍无法显示，先检查 `pay.ldxp.cn` 自己是否返回 `X-Frame-Options` 或 `Content-Security-Policy: frame-ancestors` 限制；Sub2API 侧只能放行自己的 CSP，不能覆盖对方站点禁止被嵌入的策略。

## 升级流程

1. 从 `upstream` 获取新的官方 tag 或分支。
2. 将官方版本合并到定制 `main`，例如 `git merge --no-ff v0.1.160`。
3. 只重放必要的定制改动：
   - `/recharge` 前端页面
   - 侧边栏入口
   - i18n 文案
   - 路由登录保护
   - CSP `frame-src https://pay.ldxp.cn`
   - 预编译二进制 Dockerfile，如服务器资源不足仍需要使用
4. 本地运行前端测试、类型检查、后端 CSP 测试和生产构建。
5. 本地构建 Linux amd64 后端二进制到 `build-local/sub2api`。
6. 本地用 `Dockerfile.prebuilt-binary` 构建 `sub2api-custom:0.1.160-ui1`；Docker Hub 不可用时，用 `Dockerfile.rebase-binary` 基于 `0.1.158-ui1` 离线重建。
7. 本地执行 `docker save`、压缩并计算 SHA-256，上传到 `/opt/sub2api-images/0.1.160-ui1`。
8. 服务器校验归档后只执行 `docker load`，再备份 `docker-compose.override.yml` 并只重建 `sub2api` 应用容器。
9. 验证 `/health`、容器健康、版本信息、`/recharge`、充值页 JS 和 CSP。
10. 将升级工作流同步到 `docs/custom-upgrade-workflow.md` 并随分支 push 到 GitHub。

## 数据与停机约束

- 不要删除或重建 `/opt/sub2api-deploy/data`、`postgres_data`、`redis_data`。
- 不要执行会清空数据卷的 compose down/volume prune 操作。
- 升级时只重建 `sub2api` 应用容器，Postgres 和 Redis 应保持运行。
- 服务器内存和磁盘较紧张，固定使用本机前端构建 + 本机 Linux 二进制构建 + 本机 Docker 构建；服务器只允许校验、`docker load` 和运行镜像。
- 不要使用后台在线更新覆盖定制页面；在线更新可能回到官方镜像或官方前端，导致 `/recharge` 定制丢失。

## 回滚

若新镜像上线后异常，把 `/opt/sub2api-deploy/docker-compose.override.yml` 的镜像改回上一个可用标签 `sub2api-custom:0.1.156-ui5`，或恢复本次部署前的 `bak-v0158-ui1` 备份，然后执行：

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d sub2api
```
