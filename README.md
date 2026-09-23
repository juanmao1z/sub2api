# Sub2API

Sub2API 是一个面向开发者和团队的 AI API 网关。它将多个模型服务统一到一个管理平台中，提供账户管理、API 密钥、分组路由、用量统计和管理员控制台。

## 核心功能

- 统一接入 OpenAI、Anthropic、Gemini、Grok 等模型服务
- 通过账户、分组和 API 密钥管理模型访问权限
- 支持 OpenAI、Anthropic 等常用 API 兼容协议
- 提供请求转发、故障切换、限流和并发控制
- 提供用量、延迟、输出速率和余额等统计信息
- 提供 Web 管理后台，支持用户、账户、密钥和系统配置管理
- 使用 PostgreSQL 保存业务数据，使用 Redis 支持缓存和调度

## 快速启动

### Docker Compose

需要提前安装 Docker 和 Docker Compose。

```bash
git clone https://github.com/juanmao1z/sub2api.git
cd sub2api/deploy
cp .env.example .env
docker compose -f docker-compose.local.yml up -d
```

启动后访问：

```text
http://localhost:8080
```

首次访问时按页面中的安装向导创建管理员账户。查看服务日志：

```bash
docker compose -f docker-compose.local.yml logs -f sub2api
```

### 本地开发

本地开发需要 Go、Node.js、pnpm、PostgreSQL 和 Redis。

启动后端：

```bash
cd backend
go run ./cmd/server
```

启动前端：

```bash
cd frontend
pnpm install
pnpm run dev
```

## 配置说明

### Docker 配置

复制环境变量模板并按部署环境修改：

```bash
cd deploy
cp .env.example .env
```

常用配置项：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `SERVER_PORT` | `8080` | Web 服务端口 |
| `DATABASE_HOST` | `postgres` | PostgreSQL 地址 |
| `DATABASE_PORT` | `5432` | PostgreSQL 端口 |
| `REDIS_HOST` | `redis` | Redis 地址 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `CONFIG_FILE` | `./config.yaml` | 可选的自定义配置文件 |

### 配置文件

二进制部署或需要细化服务参数时，可以复制配置模板：

```bash
cp deploy/config.example.yaml deploy/config.yaml
```

然后根据实际环境调整服务器、数据库、Redis、网关、安全和日志配置。生产环境请妥善保管数据库密码、JWT 密钥、API 密钥等敏感信息，不要将真实凭据提交到 Git。
