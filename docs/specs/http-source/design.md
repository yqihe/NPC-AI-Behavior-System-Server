# HTTPSource 设计方案

## 方案描述

### 整体思路

HTTPSource 实现 `config.Source` 接口，启动时从 ADMIN 平台的 HTTP API 全量拉取所有配置到内存（map），之后所有读取走内存。与 JSONSource / MongoSource 对调用方完全透明。

```
启动阶段：
  ADMIN API (4 × GET) → HTTPSource.init() → 内存 map

运行阶段：
  调用方 → HTTPSource.LoadXxx() → 内存 map 读取（不调 ADMIN API）
```

### ADMIN API 约定

| 接口 | 返回格式 |
|------|---------|
| `GET /api/configs/event_types` | `{"items": [{"name": "explosion", "config": {...}}, ...]}` |
| `GET /api/configs/npc_types` | `{"items": [{"name": "civilian", "config": {...}}, ...]}` |
| `GET /api/configs/fsm_configs` | `{"items": [{"name": "civilian", "config": {...}}, ...]}` |
| `GET /api/configs/bt_trees` | `{"items": [{"name": "guard/patrol", "config": {...}}, ...]}` |

每个 item 的 `config` 字段是 JSON 对象，与现有 JSON 文件 / MongoDB 文档中的 `config` 内容一致。

### HTTPSource 结构

```go
// HTTPSource 从 ADMIN 平台 HTTP API 加载配置，启动时全量拉取到内存
type HTTPSource struct {
    npcTypes   map[string][]byte // name → raw JSON
    fsmConfigs map[string][]byte // name → raw JSON
    btTrees    map[string][]byte // name → raw JSON
    eventTypes map[string][]byte // name → raw JSON
}

func NewHTTPSource(ctx context.Context, baseURL string) (*HTTPSource, error)
```

`NewHTTPSource` 在构造时完成全部拉取：

1. 依次请求 4 个 API endpoint
2. 解析 `{"items": [...]}` 响应
3. 将每个 item 的 `config` 字段序列化为 `[]byte` 存入对应 map
4. 如果任何请求失败、返回非 200、或 items 为空，返回错误
5. 每个 HTTP 请求使用带 10s 超时的 context

### 响应解析

```go
// configItem ADMIN API 返回的配置条目
type configItem struct {
    Name   string          `json:"name"`
    Config json.RawMessage `json:"config"`
}

// configResponse ADMIN API 统一响应格式
type configResponse struct {
    Items []configItem `json:"items"`
}
```

使用 `json.RawMessage` 保持 `config` 字段为原始 JSON bytes，避免二次序列化/反序列化。

### Source 接口实现

与 MongoSource 完全一致——从内存 map 读取 `[]byte`，按需 Unmarshal：

```go
func (s *HTTPSource) LoadFSMConfig(npcType string) (*fsm.FSMConfig, error) {
    data, ok := s.fsmConfigs[npcType]
    if !ok {
        return nil, fmt.Errorf("config: FSM %q not found via ADMIN API", npcType)
    }
    var cfg fsm.FSMConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("config: parse FSM %q: %w", npcType, err)
    }
    return &cfg, nil
}

// LoadBTTree, LoadEventConfig, LoadAllEventConfigs, LoadNPCTypeConfig 同理
```

### main.go 切换逻辑

优先级：`NPC_ADMIN_API` > `NPC_MONGO_URI` > JSONSource

```go
var src config.Source
if cfg.AdminAPI != "" {
    httpSrc, err := config.NewHTTPSource(context.Background(), cfg.AdminAPI)
    if err != nil {
        slog.Error("config.http_error", "err", err)
        os.Exit(1)
    }
    src = httpSrc
    slog.Info("config.source", "type", "http", "base_url", cfg.AdminAPI)
} else if cfg.MongoURI != "" {
    // ... 现有 MongoSource 逻辑不变
} else {
    src = config.NewJSONSource("configs")
    slog.Info("config.source", "type", "json", "dir", "configs")
}
```

### 环境变量

`serverConfig` 新增字段：

```go
type serverConfig struct {
    // ... 现有字段
    AdminAPI string `json:"admin_api"` // ADMIN 平台 API 地址
}
```

`applyEnvOverrides` 新增：

```go
if v := os.Getenv("NPC_ADMIN_API"); v != "" {
    cfg.AdminAPI = v
}
```

---

## 方案对比

### 方案 A（选定）：启动时全量拉取到内存

如上所述。调用 4 个 GET 接口，全量加载到内存 map。

**优点**：
- 运行时零外部依赖——ADMIN 挂了不影响已启动的游戏服务端
- 与 JSONSource / MongoSource 行为完全一致：构造时加载，之后只读
- 只用标准库 `net/http` + `encoding/json`，无外部依赖
- 实现简单，与 MongoSource 结构高度对称

**缺点**：
- 不支持热更新（需求明确排除）
- ADMIN 必须在游戏服务端之前启动

### 方案 B（备选）：保持长连接，按需查询

每次调用 LoadXxx 时实时请求 ADMIN API。

**不选原因**：
- 运行时强依赖 ADMIN——ADMIN 抖动导致 NPC 行为异常
- 每次 SpawnNPC 都 HTTP 调用，增加延迟
- 需要 HTTP 连接池管理、超时、重试、熔断等复杂逻辑
- 配置数据极小且只读，没必要实时查询

### 方案 C（备选）：ADMIN 推送配置到游戏服务端

ADMIN 修改配置后主动 POST 到游戏服务端的管理接口。

**不选原因**：
- 游戏服务端需要暴露管理接口，增加攻击面
- 需要处理推送顺序、重试、幂等等问题
- 启动时仍然需要全量拉取，推送只解决热更新——而热更新不在本 spec 范围内
- 过度设计

---

## 红线检查

| 红线 | 状态 | 说明 |
|------|------|------|
| 禁止硬编码业务逻辑 | ✅ 不涉及 | HTTPSource 只做数据搬运 |
| 禁止破坏层次 | ✅ 符合 | config/ 不依赖 runtime/gateway/ |
| 禁止弱类型通信 | ✅ 不涉及 | 配置数据透传 raw JSON |
| 禁止安全隐患 | ✅ 符合 | HTTP 请求使用带超时的 context；不涉及文件路径拼接 |
| 禁止静默降级 | ✅ 符合 | API 失败、空列表、非 200 均报错退出，不 fallback |
| 禁止过度设计 | ✅ 符合 | 不引入 Redis、不做热更新、不做认证、只用标准库 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 运营在 ADMIN 页面创建 → 重启即加载，无需接触数据库或文件 |
| 加 NPC 类型 | **正面** | 同上 |
| NPC 间交互 | **中性** | 不涉及 |

---

## 依赖方向

```
cmd/server/main.go
    ├─→ internal/config/     (Source 接口 + JSONSource + MongoSource + HTTPSource)
    ├─→ internal/gateway/
    └─→ internal/runtime/

internal/config/http_source.go
    ├─→ net/http             (标准库)
    ├─→ encoding/json        (标准库)
    └─→ internal/core/fsm/   (FSMConfig 类型，已有依赖)
```

依赖方向单向。HTTPSource 不引入任何新的外部依赖，只用标准库。

---

## 并发安全

与 MongoSource 一致：HTTPSource 在构造时写入所有 map，构造完成后所有方法只读 map。**无并发问题**——Go 中 map 的并发读是安全的。

---

## 配置变更

### 新增环境变量

| 变量 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `NPC_ADMIN_API` | ADMIN 平台 API 基础地址，非空时使用 HTTPSource | 空 | `http://localhost:3000` |

### 修改文件

| 文件 | 变更 |
|------|------|
| `.env` | 新增 `NPC_ADMIN_API=` |
| `docker-compose.yml` | 新增 `NPC_ADMIN_API` 环境变量传递；恢复 mongo 服务（保留 MongoSource 备选） |

### 不变

- `configs/server.json` — 可选增加 `admin_api` 字段，但环境变量覆盖为主
- `config.Source` 接口 — 不修改
- `JSONSource` / `MongoSource` — 不修改

---

## 测试策略

### 单元测试（internal/config/http_source_test.go）

使用 `net/http/httptest` 模拟 ADMIN API，无需真实 ADMIN 服务。

| 测试 | 覆盖 | 验收 |
|------|------|------|
| `TestHTTPSource_LoadAll` | httptest.Server 返回完整配置 → NewHTTPSource → 验证所有 Load 方法返回正确数据 | R1, R3 |
| `TestHTTPSource_NotFound` | 加载不存在的配置名 → 返回错误 | R1 |
| `TestHTTPSource_DisconnectAfterLoad` | 加载后关闭 httptest.Server → Load 方法仍正常 | R4 |
| `TestHTTPSource_Unreachable` | 连接不存在的地址 → 返回包含 URL 的错误 | R5 |
| `TestHTTPSource_EmptyItems` | 返回 `{"items":[]}` → 返回错误 | R6 |
| `TestHTTPSource_Non200` | 返回 500 → 返回错误 | R7 |
| `TestHTTPSource_Timeout` | httptest.Server 延迟响应 → context 超时 → 返回错误 | R8 |

### e2e 测试

现有 e2e 测试不变（使用 JSONSource）。HTTPSource 的 e2e 验证通过联调手动进行。

### 回归

`go test ./...` 全部通过（HTTPSource 测试使用 httptest，无外部依赖，始终运行）。
