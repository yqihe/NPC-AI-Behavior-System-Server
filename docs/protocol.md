# WebSocket 协议规范

供 Unity 客户端和其他消费者参考的完整协议定义。

## 连接

```
ws://<host>:9820/ws
```

- 开发环境：`ws://localhost:9820/ws`
- Docker 环境：`docker compose up --build` 后同上
- 无认证，直接连接

## 消息格式

所有消息均为 JSON 文本帧，统一信封结构：

```json
{
    "type": "消息类型",
    "id": "请求ID（可选，客户端生成，响应原样返回）",
    "data": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 消息类型，见下表 |
| `id` | string | 请求 ID，客户端自行生成（如 UUID），服务端在响应中原样返回。用于匹配请求和响应 |
| `data` | object | 载荷，不同类型有不同结构 |

## 消息类型一览

| type | 方向 | 说明 |
|------|------|------|
| `spawn_npc` | C→S | 创建 NPC |
| `remove_npc` | C→S | 移除 NPC |
| `publish_event` | C→S | 发布世界事件 |
| `query_npc` | C→S | 查询 NPC 详细状态 |
| `response` | S→C | 请求成功响应 |
| `error` | S→C | 请求失败响应 |
| `world_snapshot` | S→C | 每 Tick 广播的世界状态快照 |

---

## 客户端 → 服务端

### spawn_npc — 创建 NPC

```json
{
    "type": "spawn_npc",
    "id": "req_001",
    "data": {
        "npc_id": "npc_1",
        "type_name": "civilian",
        "x": 100.0,
        "z": 200.0
    }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `npc_id` | string | 是 | NPC 唯一 ID，客户端指定 |
| `type_name` | string | 是 | NPC 类型名，对应服务端配置（如 `civilian`、`police`） |
| `x` | float64 | 是 | 初始位置 X |
| `z` | float64 | 是 | 初始位置 Z |

**成功响应**：
```json
{
    "type": "response",
    "id": "req_001",
    "data": {
        "npc_id": "npc_1",
        "type_name": "civilian"
    }
}
```

**可能的错误**：
- `npc_already_exists` — 该 ID 已存在
- `config_error` — NPC 类型配置不存在或无法解析
- `create_error` — 创建失败（FSM/BT 配置问题）

### remove_npc — 移除 NPC

```json
{
    "type": "remove_npc",
    "id": "req_002",
    "data": {
        "npc_id": "npc_1"
    }
}
```

**成功响应**：
```json
{
    "type": "response",
    "id": "req_002",
    "data": {
        "npc_id": "npc_1"
    }
}
```

**可能的错误**：
- `npc_not_found` — 该 ID 不存在

### publish_event — 发布世界事件

```json
{
    "type": "publish_event",
    "id": "req_003",
    "data": {
        "event_type": "explosion",
        "x": 150.0,
        "z": 100.0,
        "severity": 80.0,
        "source_id": "player_1"
    }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `event_type` | string | 是 | 事件类型名（`explosion`、`gunshot`、`shout`、`fire`） |
| `x` | float64 | 是 | 事件位置 X |
| `z` | float64 | 是 | 事件位置 Z |
| `severity` | float64 | 否 | 覆盖默认严重度。0 或省略则用事件类型的 default_severity |
| `source_id` | string | 否 | 事件来源实体 ID |

**成功响应**：
```json
{
    "type": "response",
    "id": "req_003",
    "data": {
        "event_id": "evt_42"
    }
}
```

**可能的错误**：
- `unknown_event_type` — 事件类型不存在

### query_npc — 查询 NPC 状态

```json
{
    "type": "query_npc",
    "id": "req_004",
    "data": {
        "npc_id": "npc_1"
    }
}
```

**成功响应**：
```json
{
    "type": "response",
    "id": "req_004",
    "data": {
        "npc_id": "npc_1",
        "type_name": "civilian",
        "x": 100.0,
        "z": 200.0,
        "fsm_state": "Flee",
        "current_action": "run_away",
        "threat_level": 60.0
    }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `npc_id` | string | NPC ID |
| `type_name` | string | NPC 类型名 |
| `x`, `z` | float64 | 当前位置 |
| `fsm_state` | string | 当前 FSM 状态（`Idle`、`Alarmed`、`Flee`、`Engage` 等） |
| `current_action` | string | BT 当前执行的动作名（可能为空） |
| `threat_level` | float64 | 当前威胁值（0-100） |

**可能的错误**：
- `npc_not_found` — 该 ID 不存在

---

## 服务端 → 客户端

### error — 错误响应

```json
{
    "type": "error",
    "id": "req_001",
    "data": {
        "code": "npc_not_found",
        "message": "NPC with id npc_99 not found"
    }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | string | 机器可读错误码 |
| `message` | string | 人类可读描述 |

**所有错误码**：

| code | 触发场景 |
|------|---------|
| `invalid_json` | 消息无法解析为 JSON |
| `invalid_data` | data 字段无法解析为对应请求结构 |
| `unknown_message_type` | type 字段不匹配任何已注册的处理器 |
| `npc_already_exists` | spawn_npc 时 ID 已存在 |
| `npc_not_found` | remove_npc / query_npc 时 ID 不存在 |
| `config_error` | NPC 类型或事件类型配置加载失败 |
| `create_error` | NPC 实例创建失败 |
| `unknown_event_type` | publish_event 时事件类型不存在 |

### world_snapshot — 世界状态广播

服务端每 Tick（默认 100ms）向所有连接的客户端广播：

```json
{
    "type": "world_snapshot",
    "data": {
        "tick": 42,
        "npcs": [
            {
                "npc_id": "npc_1",
                "type_name": "civilian",
                "x": 100.0,
                "z": 200.0,
                "fsm_state": "Flee",
                "current_action": "run_away",
                "threat_level": 60.0
            },
            {
                "npc_id": "police_1",
                "type_name": "police",
                "x": 50.0,
                "z": 50.0,
                "fsm_state": "Engage",
                "current_action": "engage_target",
                "threat_level": 80.0
            }
        ]
    }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `tick` | uint64 | 当前 Tick 序号（单调递增） |
| `npcs` | array | 所有 NPC 的状态快照，结构与 query_npc 响应相同 |

**注意**：
- 无客户端连接时不广播（节省资源）
- `npcs` 为空数组 `[]` 当没有 NPC 时（不是 null）
- 没有 `id` 字段（广播不是请求的响应）

---

## 典型交互流程

```
Client                              Server
  │                                    │
  │──── WebSocket Connect ────────────>│
  │                                    │
  │──── spawn_npc (civilian) ─────────>│
  │<─── response (ok) ────────────────│
  │                                    │
  │──── spawn_npc (police) ──────────>│
  │<─── response (ok) ────────────────│
  │                                    │
  │<─── world_snapshot (tick=1) ──────│  ← 每 100ms 自动推送
  │<─── world_snapshot (tick=2) ──────│
  │                                    │
  │──── publish_event (explosion) ───>│
  │<─── response (event_id) ─────────│
  │                                    │
  │<─── world_snapshot (tick=3) ──────│  ← NPC 状态已变化
  │     civilian: Flee                 │
  │     police: Engage                 │
  │                                    │
  │──── query_npc (npc_1) ──────────>│
  │<─── response (详细状态) ──────────│
  │                                    │
  │──── remove_npc (npc_1) ─────────>│
  │<─── response (ok) ────────────────│
  │                                    │
  │──── WebSocket Close ─────────────>│
```

## 当前可用的 NPC 类型

| type_name | 行为模式 | FSM 状态 |
|-----------|---------|----------|
| `civilian` | 遇到威胁逃跑 | Idle → Alarmed → Flee |
| `police` | 遇到威胁迎敌 | Idle → Alarmed → Engage |

## 当前可用的事件类型

| event_type | severity | TTL | 感知方式 | 范围 |
|------------|----------|-----|---------|------|
| `explosion` | 80 | 15s | auditory | 500m |
| `gunshot` | 90 | 10s | auditory | 300m |
| `shout` | 30 | 8s | auditory | 200m |
| `fire` | 60 | 20s | visual | 150m |
