# HTTPSource 需求分析

## 动机

当前游戏服务端通过 `MongoSource` 直接连接 MongoDB 加载配置。随着 ADMIN 运营平台的完成，架构出现了一个不合理的耦合：**游戏服务端和 ADMIN 平台都直接连接同一个 MongoDB 实例**。

**不做会怎样**：

- **部署耦合**：游戏服务端必须能访问 ADMIN 的 MongoDB，两个服务的网络拓扑被绑定在一起。联调时需要协调 MongoDB 端口、共享容器或指向同一实例
- **职责模糊**：MongoDB 的 schema（collection 结构、文档格式）由 ADMIN 管理，但游戏服务端也直接读取，双方对 schema 有隐式耦合。ADMIN 改了文档结构，游戏服务端就挂
- **依赖膨胀**：游戏服务端为了读配置而引入 `go.mongodb.org/mongo-driver`，这是一个重型依赖。如果改为 HTTP 调用，只需标准库 `net/http`
- **违反 roadmap 架构演进方向**：roadmap 中系统全景图显示 `游戏服务端 ←启动读← MongoDB`，但 ADMIN 平台已经是 MongoDB 的唯一写入方。让 ADMIN 同时提供读 API 是自然的演进——MongoDB 对游戏服务端来说应该是不可见的

## 优先级

**高**。ADMIN 平台已完成配置 CRUD，正在联调阶段。当前联调因 MongoDB 共享方式（端口冲突、Docker 网络）受阻。切换到 HTTP API 后：

- 游戏服务端不再需要 MongoDB 容器，`docker-compose.yml` 简化
- 联调只需一个 URL（`NPC_ADMIN_API`），无需协调数据库连接
- ADMIN 平台成为配置的单一数据源（Single Source of Truth）

## 预期效果

### 场景 1：开发环境（默认，无 ADMIN）

`NPC_ADMIN_API` 和 `NPC_MONGO_URI` 均为空 → 使用 `JSONSource`，行为与现在完全一致。

### 场景 2：联调 / 生产环境（有 ADMIN）

1. 设置 `NPC_ADMIN_API=http://admin-host:port`
2. 服务端启动时，依次调用 ADMIN 的 4 个 GET 接口，全量拉取配置到内存
3. 拉取完成后，运行时行为与 JSONSource / MongoSource 完全一致——纯内存读取
4. 如果 ADMIN API 不可达或返回错误，启动失败并报明确错误

### 场景 3：ADMIN 创建新配置后验证

1. 运营人员在 ADMIN 页面创建 guard NPC 类型 + earthquake 事件
2. 游戏服务端重启（`docker compose up --build`）
3. 启动日志显示 `config.source type=http`，事件/NPC 数量包含新增项
4. WS 发送 `spawn_npc type_name=guard` → 成功

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 状态 | 说明 |
|--------|------|------|
| `config.Source` 接口 | ✅ 已定义 | HTTPSource 实现同一接口 |
| ADMIN 平台 4 个 GET API | ✅ ADMIN 已确认可提供 | `/api/configs/{event_types,npc_types,fsm_configs,bt_trees}` |
| API 返回格式约定 | ✅ 已对齐 | `{"items": [{name, config}, ...]}` |

### 谁依赖这个

| 依赖方 | 说明 |
|--------|------|
| `cmd/server/main.go` | 根据 `NPC_ADMIN_API` 选择 Source 实现 |
| 联调流程 | 消除 MongoDB 共享依赖，简化联调步骤 |
| 生产部署 | 游戏服务端不再需要 MongoDB 连接信息 |

## 改动范围

| 包 | 预估文件数 | 说明 |
|----|-----------|------|
| `internal/config/` | 2 | `http_source.go` + `http_source_test.go` |
| `cmd/server/` | 1 | `main.go` 加 `NPC_ADMIN_API` 配置源分支 |
| 根目录 | 2 | `.env` + `docker-compose.yml` 调整 |
| `docs/` | 3 | `deployment.md` + `roadmap.md` + `CLAUDE.md` 更新 |
| 总计 | ~8 | |

**不改动**：`config.Source` 接口、`JSONSource`、`internal/core/`、`internal/runtime/`、`internal/gateway/`。

**可能移除**：`MongoSource`（`mongo_source.go` + `mongo_source_test.go`）和 `go.mongodb.org/mongo-driver` 依赖。视审批决定——如果保留 MongoSource 作为备选，则不移除。

## 扩展轴检查

不直接服务于三个扩展轴，但间接支持：

- **加事件源 / 加 NPC 类型**：运营人员在 ADMIN 页面创建配置 → 游戏服务端重启即可加载，无需接触数据库或 JSON 文件。配置创建的用户体验由 ADMIN 保证，游戏服务端只关心"启动时拿到正确数据"
- 强化了"加配置不改代码"原则——连配置文件都不用碰了

## 验收标准

| 编号 | 验收标准 | 验证方式 |
|------|---------|---------|
| R1 | `HTTPSource` 实现 `config.Source` 接口全部 5 个方法 | 编译通过 + `var _ Source = (*HTTPSource)(nil)` |
| R2 | `NPC_ADMIN_API` 和 `NPC_MONGO_URI` 均为空时使用 `JSONSource`，行为不变 | 现有 e2e 测试全部通过 |
| R3 | `NPC_ADMIN_API` 非空时使用 `HTTPSource`，从 ADMIN API 拉取配置 | 单元测试：httptest.Server 模拟 ADMIN API → 加载成功 |
| R4 | 启动时全量拉取配置到内存，运行时不再调用 ADMIN API | 单元测试验证：拉取后关闭 httptest.Server，读取仍正常 |
| R5 | ADMIN API 不可达时启动报错退出，错误信息包含 URL 和具体错误 | 单元测试：连接不存在的地址 → 返回错误 |
| R6 | ADMIN API 返回空列表时启动报错退出 | 单元测试：返回 `{"items":[]}` → 返回错误 |
| R7 | ADMIN API 返回非 200 状态码时启动报错退出 | 单元测试：返回 500 → 返回错误 |
| R8 | HTTP 请求使用带超时的 context（≤10s） | 代码审查 |
| R9 | `.env` 包含 `NPC_ADMIN_API` 变量 | 文件审查 |
| R10 | `deployment.md`、`roadmap.md`、`CLAUDE.md` 更新反映新架构 | 文档审查 |

## 不做什么

- **不做 MongoSource 移除**（本 spec 范围内）：MongoSource 保留作为备选，后续单独评估是否移除
- **不做热更新**：启动时拉取一次，运行时不轮询 ADMIN API
- **不做认证**：内网服务间调用，不加 token（后续有需要再加）
- **不做分页**：配置量不大（预计 <100 条），全量返回
- **不做缓存**：拉取后存内存即可，不引入 Redis
- **不做 ADMIN API 的 schema 校验**：信任 ADMIN 返回的数据格式与 MongoDB 文档一致
