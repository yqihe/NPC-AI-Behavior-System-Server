# /integration — 跨项目联调（游戏服务端 ↔ ADMIN 平台）

新增 NPC 类型、事件源等配置时的双向协作流程。任一方均可发起。

## 身份声明

每次对话开始时，先声明身份和当前进度：
```
我是 [游戏服务端 CC / ADMIN CC]，当前执行 /integration 第 N 步：<简述>
```

## 角色分工

- **方案方**：谁想加谁起草 PROPOSAL（服务端想加 → 服务端起草；ADMIN 想加 → ADMIN 起草）
- **确认方**：对方审核方案，回复 CONFIRM
- **执行方**：始终是 ADMIN（在页面创建配置，写入 MongoDB）
- **验证方**：始终是游戏服务端（重启加载 + WebSocket 测试）

## 流程（4 步）

### 第 1 步：PROPOSAL（方案方 → 确认方）

```
## [PROPOSAL] <标题>

### 新增事件类型
| 名称 | 威胁等级 | TTL(秒) | 传播方式 | 范围(米) |
|------|---------|---------|---------|---------|

### 新增状态机：<名称>
- 初始状态 / 状态列表 / 转换规则（from→to, priority, 条件）

### 新增行为树
| 名称 | 描述 |

### 新增 NPC 类型：<名称>
- FSM 引用 / 感知范围 / BT 绑定（State→bt_name）

### 需确认
1. <问题>
```

**格式要求**：PROPOSAL 只用表格和文字描述，**禁止贴 JSON 原文**。仅在以下情况例外：(1) 需要新增字段/修改结构 schema 时；(2) 修 Bug 需要展示具体 JSON 差异时。

**发送前检查**：状态名首字母大写、bt_refs key 与 FSM state 一致、BB Key 在白名单、stub_action result ∈ {success,failure,running}、op ∈ {==,!=,>,>=,<,<=}、global 事件 range=0

### 第 2 步：CONFIRM（确认方 → 方案方）

```
## [CONFIRM] <标题>
### 结果：通过 / 需调整
- 事件：✅/❌
- 状态机：✅/❌
- BB Key 白名单：✅/❌
- 行为树：✅/❌
### 调整项（如有）
### 回答（对 PROPOSAL 中问题的逐条回复）
```

**服务端确认时必查**：BB Key 注册情况（`blackboard/keys.go`）、global 事件在感知层和决策层的行为、ref_key 类型匹配

确认通过 → ADMIN 执行配置写入（按顺序：事件→行为树→状态机→NPC）

### 第 2.5 步：ADMIN 执行配置写入（CONFIRM 后、READY 前）

**硬性规定：所有配置变更必须通过 REST API 写入 MongoDB，禁止只修改 `configs/` 目录下的本地参考文件。**

`configs/` 目录是离线参考，游戏服务端通过 `/api/configs/*` 从 MongoDB 读取数据。只改本地文件不会影响 MongoDB，服务端 sync 拉不到变更。

**执行步骤**：
1. **创建**被引用项（先创建 BT tree，再创建 FSM，因为 NPC type 会校验引用是否存在）
   - 新 BT tree → `POST /api/v1/bt-trees`
   - 新 FSM → `POST /api/v1/fsm-configs`
   - 新事件类型 → `POST /api/v1/event-types`
2. **更新**已有配置
   - 改 FSM → `PUT /api/v1/fsm-configs/{name}`
   - 改 NPC type → `PUT /api/v1/npc-types/{name}`
3. **验证**：每次 API 调用后检查返回的 JSON 是否包含预期变更
4. **同步 `configs/` 参考文件**（可选但推荐，保持参考文件与 MongoDB 一致）

**禁止事项**：
- ❌ 只修改 `configs/*.json` 就回复 READY
- ❌ 跳过 API 返回值检查
- ❌ 先更新引用方（NPC type）再创建被引用项（BT tree）——会被校验拦截

### 第 3 步：READY（ADMIN → 服务端）

```
## [READY] <标题>
| Collection | 新增 | 总数 |
|-----------|------|------|
### API 地址：http://<host>:<port>/api/configs/
```

### 第 4 步：RESULT（服务端 → ADMIN）

服务端收到 READY 后，**必须先执行配置同步**再验证：

```bash
go run ./cmd/sync -api http://<ADMIN地址>
```

同步完成后再运行测试：

```
## [RESULT] <标题>
### 结果：PASS / FAIL
| 验证项 | 预期 | 实际 | 状态 |
|-------|------|------|------|
### 发现的问题（如有，用 BUG 格式）
```

---

## Bug 处理

联调中发现问题时，任一方发送：

```
## [BUG] <编号> <标题>
归属：ADMIN / 服务端 / 待定 | 严重度：阻塞 / 严重 / 一般
现象：<操作→结果>  预期：<应该是什么>
根因：<如已定位>
```

修复后发送：

```
## [FIXED] <编号> <标题>
修复：<改了什么>  文件：<路径>
文档更新：red-lines / pitfalls / dev-rules（哪些更新了）
验证方式：<对方怎么确认>
```

**每个 FIXED 必须检查文档同步**：新禁令→red-lines、新坑→pitfalls、新教训→dev-rules。
