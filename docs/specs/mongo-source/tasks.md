# MongoSource 任务拆解

## [x] T1: 实现 MongoSource (R1, R3, R4, R5, R6)

**文件**：
- `internal/config/mongo_source.go`

**产出**：
- `MongoSource` 结构体：4 个 `map[string][]byte` 内存缓存
- `NewMongoSource(ctx, uri, database) (*MongoSource, error)`：连接 MongoDB → 从 4 个 collection 全量加载 → 存入内存 map → 断开连接
- 实现 `config.Source` 接口全部 5 个方法（纯内存读取）
- `var _ Source = (*MongoSource)(nil)` 编译期接口校验
- 连接失败、collection 为空时返回明确错误

**做完了是什么样**：
- `go build ./internal/config/` 编译通过
- 引入 `go.mongodb.org/mongo-driver/v2` 依赖

---

## [x] T2: MongoSource 集成测试 (R3, R4, R5, R6)

**文件**：
- `internal/config/mongo_source_test.go`

**产出**：
- `TestMongoSource_LoadAll`：插入测试数据 → NewMongoSource → 验证 5 个 Load 方法
- `TestMongoSource_NotFound`：查询不存在的 name → 返回错误
- `TestMongoSource_ConnectError`：错误连接串 → 返回错误
- `TestMongoSource_MemoryOnly`：加载后读取不依赖 MongoDB
- 所有测试用 `testing.Short()` 跳过（无 MongoDB 时）

**做完了是什么样**：
- 有 MongoDB 时 `go test ./internal/config/ -v` 全部通过
- 无 MongoDB 时 `go test -short ./internal/config/` 跳过 Mongo 测试，其他测试正常

---

## [x] T3: main.go 配置源切换 + .env.prod (R2, R8)

**文件**：
- `cmd/server/main.go`
- `.env.prod`

**产出**：

`main.go`：
- `mongo_uri` 非空 → `config.NewMongoSource(ctx, mongoURI, "npc_ai")` → 失败则 `os.Exit(1)`
- `mongo_uri` 为空 → `config.NewJSONSource("configs")`（现有逻辑）
- 日志标明配置源类型

`.env.prod`：
- `NPC_LOG_LEVEL=info`、`NPC_LOG_FORMAT=json`、`NPC_MONGO_URI=mongodb://mongo:27017/npc_ai`

**做完了是什么样**：
- `mongo_uri` 为空时现有 e2e 测试全部通过（行为不变）
- `mongo_uri` 非空但 MongoDB 无数据时启动报错退出

---

## [x] T4: 配置导入脚本 (R7)

**文件**：
- `scripts/import_configs.go`

**产出**：
- 读取 `configs/events/*.json` → upsert 到 `event_types` collection
- 读取 `configs/npc_types/*.json` → upsert 到 `npc_types` collection
- 读取 `configs/fsm/*.json` → upsert 到 `fsm_configs` collection
- 递归读取 `configs/bt_trees/**/*.json` → upsert 到 `bt_trees` collection（name 含子目录路径如 `civilian/idle`）
- 命令行参数：`-uri`、`-db`、`-dir`
- 每条 upsert 打印结果

**做完了是什么样**：
- `go run scripts/import_configs.go -uri=mongodb://localhost:27017 -db=npc_ai -dir=configs` 执行成功
- MongoDB 中 4 个 collection 有数据
- 重复执行幂等（upsert）

---

## 依赖顺序

```
T1 (MongoSource 实现)
 ↓
T2 (集成测试)
 ↓
T3 (main.go 切换 + .env.prod)
 ↓
T4 (导入脚本)
```

T1 必须最先。T2 验证 T1。T3 依赖 T1。T4 独立但需要 MongoDB schema 与 T1 一致。
