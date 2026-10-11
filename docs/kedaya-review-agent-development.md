# Kedaya Review 基座开发手册

本手册是以后 agent 开发 `sub2api-custom` 前端时的执行规范。当前仓库的活动前端基座是 `kedaya-review/public` 的编译版本，生产运行时通过 Sub2API 适配层接入真实后端接口。

## 1. 架构边界

- `frontend/kedaya/base/`：唯一的编译 UI 基座，来源为 `kedaya-review/public/`。
- `frontend/kedaya/base-manifest.json`：基座资源哈希清单，禁止手工修改。
- `frontend/kedaya/assemble.mjs`：从基座生成嵌入式生产前端，并应用 Sub2API 合同适配。
- `frontend/kedaya/runtime/`：生产启动、路由和资源加载运行时。
- `frontend/kedaya/sync-base.mjs`：从本地 `kedaya-review` 同步基座资源。
- `frontend/kedaya/verify.mjs`：构建后的资源、路由、适配和 review-only 标记验证。
- `backend/internal/web/dist/`：生成目录，不提交到 Git，由 Go embed 打进后端镜像。
- `archives/`：原始 `sub2api-custom` 代码归档，用于回滚和取回旧实现。

`kedaya-review/server.mjs`、`fixtures.mjs`、`/review/session`、演示账号、合成数据、截图和证据文件不属于生产前端，禁止复制到 `sub2api-custom` 的运行时。

## 2. 标准开发流程

每次修改前执行：

```bash
cd /home/zz/workspace/sub2api_dev
cat workspace.json
git -C sub2api-custom status --short --branch
git -C sub2api-custom log --oneline -n 5
```

同步新的 review 编译版本：

```bash
cd /home/zz/workspace/sub2api_dev/sub2api-custom/frontend
KEDAYA_REVIEW_ROOT=../../kedaya-review corepack pnpm run sync:base
```

如果 review 项目不在默认相邻目录，使用绝对路径：

```bash
KEDAYA_REVIEW_ROOT=/path/to/kedaya-review corepack pnpm run sync:base
```

同步脚本只复制：

- `public/assets/`
- `public/canvas/`（如果存在）
- `public/index.original.html`，保存为基座的 `index.html`
- `evidence/asset-manifest.json`，保存为 `base-manifest.json`

它不会复制 review 服务端、fixture、会话、证据页面或本地演示数据。

## 3. 构建与验证

在 `sub2api-custom/frontend` 执行：

```bash
corepack pnpm run build
corepack pnpm run typecheck
corepack pnpm run lint:check
git -C .. diff --check
```

`build` 必须输出 196 个编译资源和 8 个 Sub2API 生产适配补丁。`typecheck` 和 `lint:check` 当前运行 `kedaya/verify.mjs`，不再对已归档的旧 Vue 源码执行 Vite/Vue 类型检查。

本地开发预览：

```bash
cd /home/zz/workspace/sub2api_dev/sub2api-custom/frontend
VITE_DEV_PROXY_TARGET=http://127.0.0.1:8080 corepack pnpm run dev
```

默认地址为 `http://127.0.0.1:3000`。开发服务器会先组装基座，并将 `/api`、`/v1`、`/setup` 转发到 `VITE_DEV_PROXY_TARGET`。

## 4. 生产适配规则

允许修改的生产适配必须集中在 `frontend/kedaya/assemble.mjs`，并满足：

- 认证、权限、feature flag、支付、账单、路由和 API 路径保持 Sub2API 现有合同。
- 不将 captured-site 域名、默认支付网关、演示凭据或 mock API 带入输出。
- 所有资源必须通过 `base-manifest.json` 校验。
- 编译资源文件名变化时，先更新同步基座，再重新确认 `assemble.mjs` 的补丁锚点；禁止用模糊字符串替换。
- 不直接修改 `backend/internal/web/dist/`，它是生成产物。
- 不删除 PostgreSQL、Redis、leaderboard 或其他后端业务代码来解决前端构建问题。

## 5. 代理与网络操作

GitHub 网络操作优先使用本机代理 `http://127.0.0.1:10808`，只对当前命令生效，不写入全局配置：

```bash
PROXY=http://127.0.0.1:10808
git -C /home/zz/workspace/sub2api_dev/sub2api-custom \
  -c http.proxy="$PROXY" -c https.proxy="$PROXY" \
  fetch origin main
```

推送前先确认远程没有新提交：

```bash
PROXY=http://127.0.0.1:10808
git -C /home/zz/workspace/sub2api_dev/sub2api-custom \
  -c http.proxy="$PROXY" -c https.proxy="$PROXY" \
  ls-remote --heads origin main
```

遇到 GitHub 超时、TLS 或 IPv6 连接问题时，重试当前命令时带上上述代理。不要把代理地址、密码、token、SSH 私钥或 `.env` 写入仓库。

## 6. 提交与上传

提交前：

```bash
cd /home/zz/workspace/sub2api_dev/sub2api-custom
corepack pnpm --dir frontend run build
git diff --check
git status --short --branch
```

确认变更只包含预期内容后：

```bash
git add frontend .gitignore docs archives
git diff --cached --check
git commit -m "feat(frontend): adopt Kedaya review base"
```
推送到 GitHub：

```bash
PROXY=http://127.0.0.1:10808
git -c http.proxy="$PROXY" -c https.proxy="$PROXY" fetch origin main
git -c http.proxy="$PROXY" -c https.proxy="$PROXY" push origin main
git -c http.proxy="$PROXY" -c https.proxy="$PROXY" ls-remote --heads origin main
```

如果 `fetch` 发现远程 `main` 有本地未知提交，停止推送并先检查差异；不要强制推送，不要执行 `reset --hard` 或覆盖他人提交。

## 7. 发布边界

- GitHub 推送不等于生产部署。
- 生产部署必须有单独的明确批准。
- 发布前必须重新构建 `linux/arm64` 镜像并检查镜像架构、版本、revision 和健康状态。
- 部署只允许切换 `sub2api`，必须保留 PostgreSQL、Redis、`sub2api-leaderboard` 和数据目录。
- 失败时使用部署脚本提供的回滚目录恢复旧应用镜像。

## 8. Agent 完成标准

一个前端基座变更只有同时满足以下条件才算完成：

- 基座资源来自 `kedaya-review/public` 且 manifest 哈希全部通过。
- 生产构建成功，输出只使用 review 应用入口。
- `corepack pnpm run typecheck` 通过。
- `git diff --check` 通过。
- 没有 `LOCAL_DEMO`、`review.local`、`/review/session`、`demoUser` 或 captured-site 生产标记。
- 没有未授权的后端 API、权限、支付或数据库合同变更。
- 提交前工作区状态和 GitHub 远程状态已明确记录。
- 若执行了生产部署，必须记录镜像、健康检查、保留服务容器 ID 和回滚路径。
