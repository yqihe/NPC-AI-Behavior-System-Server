# MongoSource 设计方案

## 方案描述

### 整体思路

MongoSource 实现 `config.Source` 接口，启动时从 MongoDB 全量加载所有配置到内存（map），之后所有读取走内存。与 JSONSource 对调用方完全透明——上层代码不感知配置来源。

```
启动阶段：
  MongoDB collections → MongoSource.init() → 内存 map

运行阶段：
  调用方 → MongoSource.LoadXxx() → 内存 map 读取（不查 MongoDB）
```

### MongoDB 数据模型

复用 JSON 文件的结构，每个 collection 对应一类配置：

| Collection | 文档结构 | 对应 JSON 目录 |
|------------|---------|---------------|
| `npc_types` | `{name: "civilian", config: {...}}` | `configs/npc_types/` |
| `fsm_configs` | `{name: "civilian", config: {...}}` | `configs/fsm/` |
| `bt_trees` | `{name: "civilian/idle", config: {...}}` | `configs/bt_trees/` |
| `event_types` | `{name: "explosion", config: {...}}` | `configs/events/` |

每个文档统一结构：

```json
{
    "_id": ObjectId,
    "name": "civilian",
    "config": { ... }  // 与 JSON 文件内容完全一致
}
```

`name` 字段是查找键，与 JSONSource 中的文件名（不含 .json）对应。bt_trees 的 name 包含路径（如 `"civilian/idle"`）。

### MongoSource 结构

```go
type MongoSource struct {
    npcTypes   map[string][]byte   // name → raw JSON
    fsmConfigs map[string][]byte   // name → raw JSON  
    btTrees    map[string][]byte   // name → raw JSON
    eventTypes map[string][]byte   // name → raw JSON
}

func NewMongoSource(ctx context.Context, uri, database string) (*MongoSource, error)
```

`NewMongoSource` 在构造时完成全部加载：
1. 连接 MongoDB
2. 从 4 个 collection 读取所有文档
3. 将每个文档的 `config` 字段序列化为 `[]byte` 存入对应 map
4. 断开 MongoDB 连接（配置已在内存，不需要保持长连接）
5. 如果任何 collection 为空或连接失败，返回错误

### Source 接口实现

```go
func (s *MongoSource) LoadFSMConfig(npcType string) (*fsm.FSMConfig, error) {
    data, ok := s.fsmConfigs[npcType]
    if !ok {
        return nil, fmt.Errorf("config: FSM %q not found in MongoDB", npcType)
    }
    var cfg fsm.FSMConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("config: parse FSM %q: %w", npcType, err)
    }
    return &cfg, nil
}

func (s *MongoSource) LoadBTTree(treeName string) ([]byte, error) {
    data, ok := s.btTrees[treeName]
    if !ok {
        return nil, fmt.Errorf("config: BT tree %q not found in MongoDB", treeName)
    }
    return data, nil
}

func (s *MongoSource) LoadEventConfig(eventType string) ([]byte, error) {
    data, ok := s.eventTypes[eventType]
    if !ok {
        return nil, fmt.Errorf("config: event %q not found in MongoDB", eventType)
    }
    return data, nil
}

func (s *MongoSource) LoadAllEventConfigs() (map[string][]byte, error) {
    result := make(map[string][]byte, len(s.eventTypes))
    for name, data := range s.eventTypes {
        result[name] = data
    }
    return result, nil
}

func (s *MongoSource) LoadNPCTypeConfig(npcType string) ([]byte, error) {
    data, ok := s.npcTypes[npcType]
    if !ok {
        return nil, fmt.Errorf("config: NPC type %q not found in MongoDB", npcType)
    }
    return data, nil
}
```

### main.go 切换逻辑

```go
// 初始化配置源
var src config.Source
if cfg.MongoURI != "" {
    mongoSrc, err := config.NewMongoSource(ctx, cfg.MongoURI, "npc_ai")
    if err != nil {
        slog.Error("config.mongo_error", "err", err)
        os.Exit(1)
    }
    src = mongoSrc
    slog.Info("config.source", "type", "mongodb", "uri", cfg.MongoURI)
} else {
    src = config.NewJSONSource("configs")
    slog.Info("config.source", "type", "json", "dir", "configs")
}
```

### 配置导入脚本

`scripts/import_configs.go`：读取 `configs/` 目录下所有 JSON 文件，插入对应 MongoDB collection。

```go
// 用法：go run scripts/import_configs.go -uri=mongodb://localhost:27017 -db=npc_ai -dir=configs
```

逻辑：
1. 遍历 `configs/events/*.json` → 插入 `event_types` collection
2. 遍历 `configs/npc_types/*.json` → 插入 `npc_types` collection
3. 遍历 `configs/fsm/*.json` → 插入 `fsm_configs` collection
4. 遍历 `configs/bt_trees/**/*.json` → 插入 `bt_trees` collection（name 含子目录路径）
5. 使用 upsert（name 作为唯一键），可重复执行

### .env.prod.example 模板

```env
NPC_PORT=9820
NPC_LOG_LEVEL=info
NPC_LOG_FORMAT=json
NPC_MONGO_URI=mongodb://mongo:27017/npc_ai
MONGO_PORT=27017
```

---

## 方案对比

### 方案 A（选定）：启动时全量加载到内存，断开连接

如上所述。构造 MongoSource 时一次性读完，之后纯内存。

**优点**：
- 运行时零 MongoDB 依赖——MongoDB 挂了不影响已启动的服务
- 与 JSONSource 行为完全一致：构造时加载，之后只读
- 无连接池管理，无超时处理，无重连逻辑
- 简单可靠

**缺点**：
- 不支持热更新（需求明确排除）
- 启动时间略长（需连接 MongoDB + 查询 4 个 collection）

### 方案 B（备选）：保持长连接，按需查询

每次调用 LoadXxx 时实时查 MongoDB。

**不选原因**：
- 运行时强依赖 MongoDB——MongoDB 抖动导致 NPC 行为异常
- 每次 SpawnNPC 都查 MongoDB，增加延迟
- 需要连接池管理、超时、重试、熔断等复杂逻辑
- 配置数据极小且只读，没必要实时查询

### 方案 C（备选）：MongoDB + Redis 缓存

MongoDB 存储 → Redis 缓存 → 内存读取。

**不选原因**：
- 红线禁止："禁止在没有使用场景时提前接入 Redis"
- 配置数据量极小（几 KB），内存缓存足够
- 多加一层只增加复杂度，不增加价值

---

## 红线检查

| 红线 | 状态 | 说明 |
|------|------|------|
| 禁止硬编码业务逻辑 | ✅ 不涉及 | MongoSource 只做数据搬运 |
| 禁止破坏层次 | ✅ 符合 | config/ 不依赖 runtime/gateway/ |
| 禁止弱类型通信 | ✅ 不涉及 | 配置数据透传 raw JSON |
| 禁止安全隐患 | ✅ 符合 | MongoSource 不涉及文件路径，客户端输入不直接用于 MongoDB 查询 |
| 禁止过度设计 | ✅ 符合 | 不引入 Redis，不做热更新，不做配置版本控制 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 生产环境通过 MongoDB 插入新事件类型配置，无需重建镜像 |
| 加 NPC 类型 | **正面** | 同上 |
| NPC 间交互 | **中性** | 不涉及 |

---

## 依赖方向

```
cmd/server/main.go
    ├─→ internal/config/     (Source 接口 + JSONSource + MongoSource)
    ├─→ internal/gateway/
    └─→ internal/runtime/

internal/config/
    ├─→ internal/core/fsm/   (FSMConfig 类型，已有依赖)
    └─→ go.mongodb.org/mongo-driver  (新增外部依赖)

scripts/import_configs.go
    └─→ go.mongodb.org/mongo-driver  (独立脚本)
```

依赖方向单向，config/ 不依赖 runtime/ 或 gateway/。

---

## 并发安全

MongoSource 在构造时写入所有 map，构造完成后所有方法只读 map。**无并发问题**——Go 中 map 的并发读是安全的，只有并发读写才需要锁。

---

## 配置变更

### 新增文件
- `.env.prod.example` — 生产环境变量模板

### 修改文件
- `docker-compose.yml` — 无需修改（mongo 服务已存在）
- `configs/server.json` — 无需修改（mongo_uri 字段已存在）

### MongoDB 数据库 schema

Database: `npc_ai`

| Collection | 索引 | 文档结构 |
|------------|------|---------|
| `event_types` | `name` (unique) | `{name: string, config: object}` |
| `npc_types` | `name` (unique) | `{name: string, config: object}` |
| `fsm_configs` | `name` (unique) | `{name: string, config: object}` |
| `bt_trees` | `name` (unique) | `{name: string, config: object}` |

---

## 测试策略

### 单元测试（internal/config/）

| 测试 | 覆盖 |
|------|------|
| `TestMongoSource_LoadAll` | 连接测试 MongoDB → 插入测试数据 → NewMongoSource → 验证所有 Load 方法返回正确数据 |
| `TestMongoSource_NotFound` | 加载不存在的配置名 → 返回错误 |
| `TestMongoSource_EmptyCollection` | 某个 collection 为空 → NewMongoSource 返回错误 |
| `TestMongoSource_DisconnectAfterLoad` | 加载后断开 MongoDB → Load 方法仍正常（走内存） |

单元测试使用 `testing.Short()` 跳过——需要真实 MongoDB 的测试标记为 integration test，用 `-short` 跳过。

### e2e 测试

现有 e2e 测试不变（使用 JSONSource）。MongoSource 的 e2e 验证通过 `docker compose` + 导入脚本手动进行。

### 回归

`go test ./...` 全部通过（MongoSource 测试在无 MongoDB 时自动跳过）。

---

## 外部依赖

| 库 | 版本 | 理由 |
|---|------|------|
| `go.mongodb.org/mongo-driver/v2` | latest | Go 官方 MongoDB 驱动 |
