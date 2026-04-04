# HTTPSource 任务拆解

## T1: 实现 HTTPSource 核心 (R1, R3, R4, R8) [x]

**涉及文件**：
- `internal/config/http_source.go`（新建）

**做完了是什么样**：
- `HTTPSource` 结构体，内含 4 个 `map[string][]byte`
- `NewHTTPSource(ctx, baseURL)` 函数：依次 GET 4 个 endpoint，解析 `{"items":[{name,config}]}`，填充内存 map
- 每个请求使用 `context.WithTimeout` 10s 超时
- 实现 `Source` 接口全部 5 个方法（纯内存读取）
- `var _ Source = (*HTTPSource)(nil)` 编译期校验
- API 不可达、非 200、items 为空均返回明确错误（R5, R6, R7）

## T2: HTTPSource 单元测试 (R3, R4, R5, R6, R7, R8) [x]

**涉及文件**：
- `internal/config/http_source_test.go`（新建）

**做完了是什么样**：
- 使用 `httptest.NewServer` 模拟 ADMIN API
- 7 个测试函数全部通过：LoadAll、NotFound、DisconnectAfterLoad、Unreachable、EmptyItems、Non200、Timeout
- `go test ./internal/config/ -run TestHTTPSource -v` 全绿

## T3: main.go 接入 HTTPSource (R2, R9) [x]

**涉及文件**：
- `cmd/server/main.go`

**做完了是什么样**：
- `serverConfig` 新增 `AdminAPI string` 字段
- `applyEnvOverrides` 新增 `NPC_ADMIN_API` 读取
- 配置源选择逻辑：`AdminAPI` 非空 → HTTPSource，`MongoURI` 非空 → MongoSource，否则 → JSONSource
- 启动日志输出 `config.source type=http base_url=...`
- 现有 e2e 测试不受影响（两个变量都为空时走 JSONSource）

## T4: 环境配置更新 (R9) [x]

**涉及文件**：
- `.env`
- `docker-compose.yml`

**做完了是什么样**：
- `.env` 新增 `NPC_ADMIN_API=`（默认空）
- `.env` 中 `NPC_MONGO_URI` 恢复为空（撤销联调临时改动）
- `docker-compose.yml` 新增 `NPC_ADMIN_API` 环境变量传递，恢复 mongo 服务（撤销联调临时注释）
- `docker compose config` 无报错

## T5: 文档更新 (R10) [x]

**涉及文件**：
- `docs/deployment.md`
- `docs/roadmap.md`
- `CLAUDE.md`

**做完了是什么样**：
- `deployment.md`：环境变量表新增 `NPC_ADMIN_API`；配置源切换逻辑更新为三级优先级；新增"联调环境"小节
- `roadmap.md`：系统全景图更新（游戏服务端通过 HTTP API 读取 ADMIN，不再直连 MongoDB）；运营平台状态更新为"已完成"
- `CLAUDE.md`：环境配置章节新增 `NPC_ADMIN_API` 说明

## 依赖顺序

```
T1 → T2 → T3 → T4 → T5
```

T1 和 T2 是核心实现 + 测试，T3 接入主流程，T4 更新环境配置，T5 同步文档。
