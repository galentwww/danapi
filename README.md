# DandanPlay API 中间件v2

## 项目简介
这是一个为弹弹Play API设计的中间件服务，主要用于：
- 转发并缓存弹弹Play的API请求
- 降低弹弹Play服务器负载
- 提供API访问加速
- 保持与原API完全兼容的请求结构

## 目录结构 
```
.
├── .env # 环境配置文件
├── main.go # 程序入口
├── config/
│ └── config.go # 配置管理
├── handlers/
│ └── danmaku.go # API处理器
├── services/
│ └── dandanplay.go # 弹弹Play服务封装
└── utils/
├── auth.go # API鉴权工具
└── cache.go # Redis缓存工具
```


## 功能特性
- 支持的API端点：
  - `/api/v2/search/episodes` - 搜索剧集
  - `/api/v2/comment/{id}` - 获取弹幕
  - `/api/v2/bangumi/bgmtv/{id}` - 通过 Bangumi.tv subjectId 获取番剧详情
  - `/api/v2/related/{id}` - 兼容旧版本（返回空数据）
- 弹幕响应保持弹弹Play兼容格式，主体是包含 `count` 和 `comments` 的 JSON 对象
- 当前已知前端调用会带 `withRelated=true`，该参数会影响后续弹幕快照缓存维度
- Redis缓存支持
- PostgreSQL 弹幕快照持久化，Redis miss 后先查数据库，不直接穿透上游
- 独立配置的缓存时间
- 完整的API鉴权
- 详细的操作日志

## 部署指南

### 1. 准备工作
- 安装 Docker 和 Docker Compose
- 准备好弹弹Play的AppId和AppSecret

### 2. 配置文件
创建 `.env` 文件并填写以下配置：

```
镜像配置
MIDDLEWARE_IMAGE=ghcr.io/galentwww/danapi:latest
弹弹Play API基础URL
DANDANPLAY_BASE_URL=
API鉴权配置
APP_ID=your_app_id
APP_SECRET=your_app_secret
Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
PostgreSQL配置
POSTGRES_DB=dandanplay_middleware
POSTGRES_USER=middleware
POSTGRES_PASSWORD=middleware_password
DATABASE_URL=postgres://middleware:middleware_password@postgres:5432/dandanplay_middleware?sslmode=disable
服务器配置
SERVER_PORT=8080
缓存时间配置（秒）
SEARCH_CACHE_DURATION= # 搜索结果缓存
DANMAKU_CACHE_DURATION= # 弹幕数据缓存
弹幕快照刷新配置（秒）
REDIS_SNAPSHOT_TTL=172800
DEFAULT_REFRESH_INTERVAL=86400
EMPTY_DANMAKU_REFRESH_INTERVAL=3600
REFRESH_FAILURE_RETRY_INTERVAL=1800
REFRESH_QUEUE_SIZE=100
REFRESH_WORKER_COUNT=2
CORS配置
CORS_ALLOW_ORIGINS=*
CORS_ALLOW_METHODS=GET,POST,PUT,DELETE,OPTIONS,PATCH,HEAD
CORS_ALLOW_HEADERS=Origin,Content-Type,Accept,Authorization,X-Requested-With
CORS_EXPOSE_HEADERS=
CORS_ALLOW_CREDENTIALS=false
CORS_MAX_AGE=86400
```


### 3. 部署方式

#### 方式一：使用 Docker Compose（推荐）

GitHub Actions 会在 `main` 分支和版本 tag 上构建镜像并推送到 GitHub Container Registry：

```text
ghcr.io/galentwww/danapi:latest
ghcr.io/galentwww/danapi:main
ghcr.io/galentwww/danapi:sha-<short-sha>
```

版本 tag 还会生成对应 tag 镜像，例如 `ghcr.io/galentwww/danapi:v1.0.0`。

如果首次发布后外部机器无法匿名拉取 GHCR 镜像，需要在 GitHub 仓库的 Packages 设置中把该 container package 可见性改为 Public。

1. 复制配置模板：
   ```bash
   cp .env.example .env
   ```

2. 编辑 `.env`，至少填写以下配置：
   ```bash
   MIDDLEWARE_IMAGE=ghcr.io/galentwww/danapi:latest
   APP_ID=your_app_id
   APP_SECRET=your_app_secret
   ```

   Compose 默认会把中间件连接到内置 Redis 和 PostgreSQL 服务：
   ```bash
   REDIS_HOST=redis
   REDIS_PORT=6379
   DATABASE_URL=postgres://middleware:middleware_password@postgres:5432/dandanplay_middleware?sslmode=disable
   SERVER_PORT=8080
   ```

3. 拉取镜像并启动：
   ```bash
   docker compose pull
   docker compose up -d
   ```

4. 查看日志：
   ```bash
   docker compose logs -f middleware
   ```

5. 停止服务：
   ```bash
   docker compose down
   ```

   Redis 和 PostgreSQL 数据会保存在 Docker 命名卷中。当前目录下的默认卷名是 `dandanplay-newmiddleware-bgmcors_redis-data` 和 `dandanplay-newmiddleware-bgmcors_postgres-data`；实际前缀取决于 Compose 项目名。可以用以下命令查看：
   ```bash
   docker compose config --volumes
   docker volume inspect dandanplay-newmiddleware-bgmcors_redis-data
   docker volume inspect dandanplay-newmiddleware-bgmcors_postgres-data
   ```

   如果需要删除缓存数据：
   ```bash
   docker compose down -v
   ```

> 说明：容器内不要求必须存在 `.env` 文件。程序会优先读取 `.env`，如果文件不存在，则直接使用容器环境变量。弹幕快照 PostgreSQL 是中间件独立数据库，不连接核心业务库；payload 会以 gzip 压缩字节保存。

#### 本地开发镜像

如果要在本机源码上构建镜像，可以先构建本地镜像，再用环境变量覆盖 Compose 使用的镜像名：

```bash
docker build -t dandanplay-middleware:local .
MIDDLEWARE_IMAGE=dandanplay-middleware:local docker compose up -d
```

### 4. 弹幕快照策略

- `/api/v2/comment/{id}` 先查 Redis 热缓存，再查 PostgreSQL 快照，最后才访问弹弹Play。
- PostgreSQL 中已有快照时，即使达到刷新时间，也会先返回旧快照，再尝试后台刷新。
- PostgreSQL 快照已过期时不会回填 Redis；Redis 只保存逻辑上仍 fresh 的热快照。
- 后台刷新按 `dandan_episode_id + variant_key` 去重，worker 执行前会再次检查快照是否已经 fresh，避免并发访问重复打上游。
- 首次没有快照且上游失败时返回 503。
- `withRelated=true` 会规范化为 `variant_key = v1|withRelated=1`。
- 默认刷新周期：普通弹幕 24 小时，空 `comments` 1 小时，刷新失败 30 分钟后重试。
- 弹幕接口会返回调试响应头：
  - `X-Danmaku-Cache`: `redis`、`postgres`、`upstream` 或 `stale`
  - `X-Danmaku-Variant`: 当前 `variant_key`
  - `X-Danmaku-Fetched-At`: 快照获取时间
  - `X-Danmaku-Next-Refresh-At`: 下一次允许刷新时间

#### 方式二：使用预编译的二进制文件
1. 下载对应平台的二进制文件
2. 确保 `.env` 文件与二进制文件在同一目录
3. 添加执行权限（Linux）：
   ```bash
   chmod +x dandanplay-middleware
   ```
4. 运行服务：
   ```bash
   ./dandanplay-middleware
   ```

#### 方式三：自行编译

1. 安装Go开发环境（需要Go 1.21或更高版本）

2. 克隆代码并进入项目目录

3. 安装依赖：
   ```bash
   go mod tidy
   ```

4. 编译：
   - Linux版本（在Windows/Mac上交叉编译）：
     ```bash
     # Windows CMD
     set GOOS=linux
     set GOARCH=amd64
     go build -o dandanplay-middleware

     # PowerShell
     $env:GOOS = "linux"
     $env:GOARCH = "amd64"
     go build -o dandanplay-middleware
     ```
   - 本地版本：
     ```bash
     go build -o dandanplay-middleware
     ```

5. 运行服务：
   ```bash
   ./dandanplay-middleware
   ```

## 监控和维护
- 服务启动后会输出详细的日志，包括：
  - 缓存命中/未命中情况
  - API请求状态
  - 错误信息
- 可以通过Redis命令行工具查看缓存状态：
  ```bash
  redis-cli
  keys *          # 查看所有缓存键
  ttl <key>       # 查看特定键的过期时间
  get <key>       # 查看特定键的内容
  ```

## 注意事项
1. 确保服务器时间准确，否则可能导致API鉴权失败
2. 建议使用进程管理工具（如systemd、supervisor等）管理服务
3. 根据实际需求调整缓存时间
4. 定期检查日志确保服务正常运行
