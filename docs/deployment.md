# 部署与环境规范

## 环境定义

| 环境 | 用途 | 配置源 | 日志 | MongoDB |
|------|------|--------|------|---------|
| **dev** | 本地开发、调试、单元测试 | JSON 文件（`configs/`） | text / debug | 可选（容器内） |
| **prod** | 生产部署、答辩演示 | MongoDB | json / info | 必须 |

## 环境变量

所有运行时配置通过环境变量注入，不硬编码在代码或 Dockerfile 中。

| 变量 | 说明 | dev 默认 | prod 示例 |
|------|------|---------|-----------|
| `NPC_ADDR` | WS 监听地址 | `:9820` | `:9820` |
| `NPC_LOG_LEVEL` | 日志级别 | `debug` | `info` |
| `NPC_LOG_FORMAT` | 日志格式 | `text` | `json` |
| `NPC_ADMIN_API` | ADMIN 平台 API 地址，非空时从 ADMIN 拉取配置 | 空 | `http://admin:3000` |
| `NPC_MONGO_URI` | MongoDB 连接串，`NPC_ADMIN_API` 为空时生效 | 空 | `mongodb://mongo:27017/npc_ai` |
| `NPC_PORT` | 宿主机映射端口 | `9820` | `9820` |
| `MONGO_PORT` | MongoDB 宿主机映射端口 | `27017` | `27017` |

## 文件规范

| 文件 | 用途 | Git 跟踪 |
|------|------|---------|
| `.env` | dev 环境默认值 | 是 |
| `.env.prod.example` | prod 环境模板 | 是 |
| `.env.prod` | prod 实际值（可能含密码） | **否**（gitignore） |
| `configs/server.json` | 服务端启动配置（addr、tick_rate 等） | 是 |
| `configs/events/*.json` | 事件类型配置 | 是 |
| `configs/npc_types/*.json` | NPC 类型配置 | 是 |
| `configs/fsm/*.json` | FSM 状态机配置 | 是 |
| `configs/bt_trees/**/*.json` | 行为树配置 | 是 |
| `Dockerfile` | 多阶段构建 | 是 |
| `docker-compose.yml` | 服务编排 | 是 |

## dev 环境

### 启动

```bash
# 启动全部服务（代码改动后必须加 --build）
docker compose up --build

# 后台启动
docker compose up --build -d

# 仅启动 MongoDB（用于集成测试）
docker compose up -d mongo

# 查看日志
docker compose logs -f server

# 停止
docker compose down
```

### 配置源切换逻辑（三级优先级）

1. `NPC_ADMIN_API` 非空 → 使用 `HTTPSource`，启动时从 ADMIN 平台 HTTP API 全量拉取配置到内存
2. `NPC_MONGO_URI` 非空 → 使用 `MongoSource`，启动时从 MongoDB 全量加载配置到内存
3. 两者均为空（默认）→ 使用 `JSONSource`，从 `configs/` 目录读取 JSON 文件

### 本地测试

```bash
# 全部测试（不需要 MongoDB）
go test ./...

# 跳过 MongoDB 集成测试
go test -short ./...

# e2e 测试（自启服务，不需要 Docker）
go test ./test/e2e/... -v

# MongoDB 集成测试（需要先启动 MongoDB）
docker compose up -d mongo
go test ./internal/config/ -v -run TestMongo
```

## 联调环境（与 ADMIN 平台）

### 启动

```bash
# ADMIN 平台已启动（如 http://localhost:3000），设置环境变量指向 ADMIN
NPC_ADMIN_API=http://host.docker.internal:3000 docker compose up --build
```

### 工作流

1. 运营人员在 ADMIN 页面创建/修改配置（NPC 类型、事件、FSM、BT）
2. 游戏服务端重启：`docker compose up --build`
3. 启动日志输出 `config.source type=http`，确认从 ADMIN 拉取
4. 通过 WS 验证新配置生效

### 注意

- 游戏服务端不直接连 MongoDB，所有配置通过 ADMIN API 获取
- ADMIN 必须在游戏服务端启动前可达，否则启动失败
- 配置只在启动时拉取一次，运行时不轮询

## prod 环境

### 首次部署

```bash
# 1. 从模板创建 prod 环境文件
cp .env.prod.example .env.prod
# 编辑 .env.prod，修改密码等敏感信息

# 2. 启动 MongoDB
docker compose --env-file .env.prod up -d mongo

# 3. 导入配置到 MongoDB
go run scripts/import_configs.go \
  -uri=mongodb://localhost:27017 \
  -db=npc_ai \
  -dir=configs

# 4. 启动全部服务
docker compose --env-file .env.prod up --build -d
```

### 更新配置

```bash
# 1. 修改 configs/ 下的 JSON 文件（git 跟踪变更）
# 2. 重新导入到 MongoDB
go run scripts/import_configs.go \
  -uri=mongodb://localhost:27017 \
  -db=npc_ai \
  -dir=configs

# 3. 重启服务（MongoSource 在启动时全量加载）
docker compose --env-file .env.prod up --build -d
```

### 更新代码

```bash
# 拉取最新代码
git pull

# 重新构建并重启（--build 确保使用最新代码）
docker compose --env-file .env.prod up --build -d
```

## dev → prod 差异检查清单

切换到 prod 前确认：

- [ ] `.env.prod` 中 `NPC_ADMIN_API` 或 `NPC_MONGO_URI` 已配置（二选一）
- [ ] `.env.prod` 中 `NPC_LOG_LEVEL=info`、`NPC_LOG_FORMAT=json`
- [ ] MongoDB 已启动且可连接
- [ ] 配置已导入 MongoDB（`scripts/import_configs.go`）
- [ ] `docker compose --env-file .env.prod up --build` 启动成功
- [ ] `docker compose logs -f server` 无 ERROR 日志
- [ ] WS 连接测试通过（`wscat -c ws://localhost:9820/ws`）

## 禁止事项

- **禁止**在 `.env.prod` 中提交密码或敏感信息到 Git
- **禁止**在 Dockerfile 中硬编码环境变量值
- **禁止**在 prod 环境使用 `debug` 日志级别（日志量过大）
- **禁止**在 prod 环境使用 `text` 日志格式（不便于日志采集工具解析）
- **禁止**跳过配置导入直接启动 prod 服务（MongoSource 加载空 collection 会启动失败）
