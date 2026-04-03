# MongoSource 需求分析

## 动机

当前系统只有 `JSONSource` 一种配置源实现，所有配置从本地 JSON 文件加载。`config.Source` 接口已预留抽象，但 `MongoSource` 从未实现。

`main.go` 中 `mongo_uri` 字段已存在但是死代码——读了环境变量却从未使用。

**不做会怎样**：
- 生产环境无法用 MongoDB 存配置，只能部署 JSON 文件到容器内
- 无法热更新配置（改 JSON 需要重新构建镜像）
- dev/prod 环境切换是假的——`mongo_uri` 设了也不生效
- 违反项目架构决策："开发 JSON + 生产 MongoDB，Loader 层抽象数据源"

## 优先级

**中**。系统功能层已全部完成，这是生产部署的前置条件。毕设答辩前需要能跑起完整的 dev/prod 双环境。

## 预期效果

### 场景 1：开发环境（默认）

`mongo_uri` 为空 → 使用 `JSONSource`，行为与现在完全一致。

### 场景 2：生产环境

1. 启动时设置 `NPC_MONGO_URI=mongodb://mongo:27017/npc_ai`
2. 服务端连接 MongoDB，从指定 database 的 collections 加载配置
3. 配置加载到内存后，后续读取全走内存，与 JSONSource 性能一致
4. 如果 MongoDB 连接失败或配置不存在，启动失败并报明确错误

### 场景 3：配置初始化

提供工具/脚本将 `configs/` 目录下的 JSON 文件导入 MongoDB，用于首次部署。

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 状态 | 说明 |
|--------|------|------|
| `config.Source` 接口 | 已定义 | MongoSource 实现同一接口 |
| `docker-compose.yml` mongo 服务 | 已配置 | MongoDB 容器已在 compose 中 |
| `main.go` mongo_uri 读取 | 已存在 | 环境变量覆盖已实现，缺切换逻辑 |

### 谁依赖这个

| 依赖方 | 说明 |
|--------|------|
| `cmd/server/main.go` | 根据 mongo_uri 选择 Source 实现 |
| 生产环境部署 | MongoDB 配置源是生产部署前提 |

## 改动范围

| 包 | 预估文件数 | 说明 |
|----|-----------|------|
| `internal/config/` | 2 | `mongo_source.go` + `mongo_source_test.go` |
| `cmd/server/` | 1 | main.go 加配置源切换逻辑 |
| `scripts/` | 1 | 配置导入脚本 `import_configs.go` |
| `docker-compose.yml` | 1 | 可能调整 mongo 服务（初始化脚本） |
| 总计 | 4-5 | |

**不改动**：`config.Source` 接口、`JSONSource`、`internal/core/`、`internal/runtime/`、`internal/gateway/`。

## 扩展轴检查

不直接服务于三个扩展轴，但间接支持：
- 加 NPC 类型/事件源时，生产环境通过 MongoDB 添加配置而非修改容器内文件
- MongoDB 支持热更新，未来可以不重启就加载新配置

## 验收标准

| 编号 | 验收标准 | 验证方式 |
|------|---------|---------|
| R1 | `MongoSource` 实现 `config.Source` 接口全部 5 个方法 | 编译通过 + `var _ Source = (*MongoSource)(nil)` |
| R2 | `mongo_uri` 为空时使用 `JSONSource`，行为不变 | 现有 e2e 测试全部通过 |
| R3 | `mongo_uri` 非空时使用 `MongoSource`，从 MongoDB 加载配置 | 单元测试：mock MongoDB → 加载成功 |
| R4 | 启动时全量加载配置到内存，运行时不再查询 MongoDB | 单元测试验证：加载后断开 MongoDB，读取仍正常 |
| R5 | MongoDB 连接失败时启动报错退出，错误信息明确 | 单元测试：错误连接串 → 返回错误 |
| R6 | MongoDB 中配置缺失时启动报错退出，错误信息明确 | 单元测试：空 collection → 返回错误 |
| R7 | 提供配置导入脚本，将 `configs/` 下的 JSON 文件导入 MongoDB | 手动验证：运行脚本 → MongoDB 中有数据 |
| R8 | `.env.prod.example` 模板存在，包含生产环境变量（mongo_uri、log_level=info 等） | 文件审查 |

## 不做什么

- **不做热更新**：启动时加载一次，运行时不监听 MongoDB 变更。热更新是未来需求
- **不做 Redis 缓存**：配置加载到内存即可，不需要 Redis 中转
- **不做配置管理 UI**：通过脚本导入，不做 Web 管理界面
- **不做配置版本控制**：MongoDB 中不维护配置历史，JSON 文件已有 git 版本控制
- **不做配置校验迁移**：MongoSource 假设 MongoDB 中的数据格式与 JSON 文件一致
