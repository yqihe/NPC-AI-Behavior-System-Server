# ADMIN API 导出快照（2026-04-18）

本文档是 PR #17 / #18 合并后、PR #18 合并（2026-04-18 夜）当日对 ADMIN 平台 `http://localhost:9821/api/configs/*` 4 个端点的真实响应抓取。

**用途**：联调历史证据，标注双边数据与契约的分歧点供后续治理参考。

**非用途**：不作为 Go 测试的回归 fixture——fixture 应锚在服务端期望契约形态，不锚在 ADMIN 现状。

> ⚠️ = 服务端目前无法消费或会运行异常的数据点

---

## 1. `GET /api/configs/event_types`

```json
{
  "items": [
    {
      "name": "evt_player_attack",
      "config": {
        "range": 20,
        "default_ttl": 5,
        "source_type": "player",
        "display_name": "玩家攻击",
        "perception_mode": "visual",
        "default_severity": 7,
        "damage_multiplier": 1.5
      }
    },
    {
      "name": "evt_loud_noise",
      "config": {
        "range": 40,
        "area_radius": 15,
        "default_ttl": 8,
        "source_type": "environment",
        "display_name": "巨大声响",
        "perception_mode": "auditory",
        "default_severity": 5
      }
    },
    {
      "name": "evt_ally_death",
      "config": {
        "range": 0,
        "default_ttl": 15,
        "source_type": "npc",
        "display_name": "同伴死亡",
        "perception_mode": "global",
        "default_severity": 9,
        "damage_multiplier": 2
      }
    }
  ]
}
```

### 分歧点

- ⚠️ **所有 config 缺 `name` 字段**：服务端 [cmd/server/main.go:210](../../cmd/server/main.go#L210) `evtTypes[cfg.Name] = &cfg` 用**内层** `cfg.Name` 做 map key。这里 3 个 config 都没 name，解析后 `cfg.Name=""`，3 条记录都塌缩到空串 key，只有 map 最后写入的那条（遍历顺序不定）会留下。
  - **修复方向**：ADMIN 在 config 内补 name，或服务端改用外层 `items[].name`
- `evt_loud_noise.config.area_radius=15`：服务端只读 `range` 字段（`EventTypeConfig.Range`），`area_radius` 是 ADMIN EventTypeSchema 给 auditory 加的扩展字段，透明忽略
- `evt_player_attack.config.damage_multiplier` / `evt_ally_death.config.damage_multiplier` / `source_type`：均为扩展字段，服务端透明忽略
- `evt_ally_death.config.range=0` + `perception_mode=global`：global 模式下 range 不参与计算（[perception.go:27](../../internal/runtime/perception/perception.go#L27) 直接 return severity），这个 0 不会引发问题

---

## 2. `GET /api/configs/fsm_configs`

```json
{
  "items": [
    {
      "name": "fsm_combat_basic",
      "config": {
        "states": [
          {"name": "idle"}, {"name": "patrol"}, {"name": "chase"},
          {"name": "attack"}, {"name": "flee"}, {"name": "dead"}
        ],
        "transitions": [
          {"from": "idle",   "to": "patrol", "priority": 0, "condition": {}},
          {"from": "patrol", "to": "chase",  "priority": 0, "condition": {"op": ">", "key": "perception_range", "value": 0}},
          {"from": "chase",  "to": "attack", "priority": 0, "condition": {"op": ">", "key": "attack_power",     "value": 0}},
          {"from": "attack", "to": "chase",  "priority": 0, "condition": {}},
          {"from": "chase",  "to": "idle",   "priority": 0, "condition": {}},
          {"from": "attack", "to": "flee",   "priority": 0, "condition": {"op": "<",  "key": "max_hp", "value": 20}},
          {"from": "flee",   "to": "idle",   "priority": 0, "condition": {}},
          {"from": "idle",   "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "patrol", "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "chase",  "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "attack", "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "flee",   "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}}
        ],
        "initial_state": "idle"
      }
    },
    {
      "name": "fsm_passive",
      "config": {
        "states": [{"name": "idle"}, {"name": "wander"}, {"name": "flee"}, {"name": "dead"}],
        "transitions": [
          {"from": "idle",   "to": "wander", "priority": 0, "condition": {}},
          {"from": "wander", "to": "idle",   "priority": 0, "condition": {}},
          {"from": "idle",   "to": "flee",   "priority": 0, "condition": {"op": "<",  "key": "max_hp", "value": 50}},
          {"from": "wander", "to": "flee",   "priority": 0, "condition": {"op": "<",  "key": "max_hp", "value": 50}},
          {"from": "flee",   "to": "idle",   "priority": 0, "condition": {}},
          {"from": "idle",   "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "wander", "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}},
          {"from": "flee",   "to": "dead",   "priority": 0, "condition": {"op": "<=", "key": "max_hp", "value": 0}}
        ],
        "initial_state": "idle"
      }
    },
    {
      "name": "guard",
      "config": {
        "states": [{"name": "patrol"}, {"name": "alert"}],
        "transitions": [],
        "initial_state": "patrol"
      }
    }
  ]
}
```

### 分歧点

- ⚠️ **`perception_range > 0` / `attack_power > 0` 条件恒为真**：这两个 key 是 NPC 静态 field，非运行时变量。所有 NPC 的 `perception_range` 都 > 0（wolf_common=20、wolf_alpha=30 等），`attack_power` 同理。NPC 启动后 `idle → patrol → chase → attack` 会在连续几个 tick 内一路 rush 完，根本不会停留
- ⚠️ **`max_hp < 20` / `max_hp < 50` 永远不触发**：`max_hp` 是 NPC 创建时写入的最大值，运行时不会变；永远不会 < 20。等价于没有 flee 转移
- ⚠️ **`max_hp <= 0` 永远不触发 dead**：同理，12 条 dead 转移都是死代码
- **根因**：ADMIN 把 NPC 静态 field 当作运行时 BB key 使用。正确方式应该是游戏服运行时维护 `current_hp` / `target_distance` 等 key；但 ADMIN 当前无"运行时 BB key 注册表"概念（所有 BB key 来自 fields.expose_bb=true）
- `guard.transitions=[]`：合法（PR #17 测试单状态 FSM 的锚点），[fsm.go](../../internal/core/fsm/fsm.go) 允许空转移列表
- `fsm_passive.flee → idle` 无条件 + `idle → flee (max_hp<50)` = 假如未来真加了 `max_hp` 动态变化的逻辑，会死循环抽搐。但在当前静态 max_hp 下不会触发（因为条件永不满足）

---

## 3. `GET /api/configs/bt_trees`

```json
{
  "items": [
    {"name": "bt/combat/idle",   "config": {"type": "sequence", "children": [
      {"type": "stub_action", "action": "wait_idle"},
      {"type": "stub_action", "action": "look_around"}
    ]}},
    {"name": "bt/combat/patrol", "config": {"type": "sequence", "children": [
      {"type": "stub_action", "action": "move_to_waypoint"},
      {"type": "stub_action", "action": "wait_at_point"}
    ]}},
    {"name": "bt/combat/chase",  "config": {"type": "selector", "children": [
      {"type": "sequence", "children": [
        {"type": "check_bb_float", "op": ">", "value": 0, "target_key": "perception_range"},
        {"type": "stub_action", "action": "move_to_target"}
      ]},
      {"type": "stub_action", "action": "search_area"}
    ]}},
    {"name": "bt/combat/attack", "config": {"type": "sequence", "children": [
      {"type": "check_bb_float", "op": ">", "key": "perception_range", "value": 0},
      {"type": "stub_action"},
      {"type": "stub_action"}
    ]}},
    {"name": "bt/passive/wander","config": {"type": "sequence", "children": [
      {"type": "stub_action", "action": "random_waypoint"},
      {"type": "stub_action", "action": "move_to_waypoint"},
      {"type": "stub_action", "action": "idle_emote"}
    ]}},
    {"name": "guard/patrol",     "config": {"type": "stub_action", "params": {"name": "patrol_action", "result": "success"}, "category": "leaf"}}
  ]
}
```

### 分歧点（BT 节点格式混乱，至少 4 种并存）

- ⚠️ **叶子节点用 `action` 字段而非 `params.name`**（[bt/combat/idle, patrol, wander](#3-get-apiconfigsbt_trees)）：服务端 `stub_action` factory 要求 `name` 在 `params` 子对象内。这些节点 factory 接收空 params，报错 `stub_action: name is required`，服务端启动 fatal
- ⚠️ **`check_bb_float` 用扁平 + 错字段名**（[bt/combat/chase](#3-get-apiconfigsbt_trees)）：`{type, op, value, target_key}` — 字段 `target_key` factory 不认（期望 `key`），且无 `params` 包裹。双重问题，parse 失败
- ⚠️ **`check_bb_float` 用扁平**（[bt/combat/attack](#3-get-apiconfigsbt_trees)）：字段名对了（`key`）但仍无 `params` 包裹。factory 接收空 params，报错 `key and op are required`
- ⚠️ **`stub_action` 空节点**（bt/combat/attack 第 2、3 子节点）：`{"type":"stub_action"}` 连 `action` 都没有，肯定挂
- ✅ **`guard/patrol` 是唯一契约干净的 BT**：`{type, params: {name, result}, category}` 格式对，`category` 是多余字段 Go `json.Unmarshal` 自动忽略，服务端能吃
- **根因**：ADMIN seed SQL 手写灌数据，绕过了 UI 的 `BtNodeEditor.vue` factory（后者输出 v2 wrapped `{type, params, children/child}`）

### 影响

任何引用 `bt/combat/*` 的 NPC（wolf_common / wolf_alpha / goblin_archer / villager_guard 的 attack/chase/idle/patrol 状态；villager_merchant 的 idle 状态）在服务端启动 `NewInstanceFromADMIN → BuildFromJSON` 阶段就会失败。ADMIN 治理前，**只有 guard_basic 一个 NPC 能端到端跑通**。

---

## 4. `GET /api/configs/npc_templates`

```json
{
  "items": [
    {
      "name": "wolf_common",
      "config": {
        "template_ref": "warrior_base",
        "fields": {
          "aggression": "aggressive", "attack_power": 18.5, "defense": 8.0,
          "is_boss": false, "loot_table": "loot_wolf_common",
          "max_hp": 120, "move_speed": 5.5, "perception_range": 20.0
        },
        "behavior": {
          "fsm_ref": "fsm_combat_basic",
          "bt_refs": {"attack": "bt/combat/attack", "chase": "bt/combat/chase", "idle": "bt/combat/idle", "patrol": "bt/combat/patrol"}
        }
      }
    },
    {"name": "wolf_alpha",        "config": {"template_ref": "warrior_base", "fields": {"aggression": "aggressive", "attack_power": 45.0, "defense": 25.0, "is_boss": true, "loot_table": "loot_wolf_alpha", "max_hp": 800, "move_speed": 6.0, "perception_range": 30.0}, "behavior": {"fsm_ref": "fsm_combat_basic", "bt_refs": {"attack": "bt/combat/attack", "chase": "bt/combat/chase", "idle": "bt/combat/idle", "patrol": "bt/combat/patrol"}}}},
    {"name": "goblin_archer",     "config": {"template_ref": "ranger_base",  "fields": {"aggression": "aggressive", "attack_power": 22.0, "defense": 3.0,  "loot_table": "loot_goblin", "max_hp": 60, "move_speed": 4.0, "perception_range": 35.0}, "behavior": {"fsm_ref": "fsm_combat_basic", "bt_refs": {"attack": "bt/combat/attack", "chase": "bt/combat/chase", "idle": "bt/combat/idle", "patrol": "bt/combat/patrol"}}}},
    {"name": "villager_merchant", "config": {"template_ref": "passive_npc",  "fields": {"aggression": "passive", "max_hp": 100, "move_speed": 2.0, "perception_range": 10.0}, "behavior": {"fsm_ref": "fsm_passive", "bt_refs": {"idle": "bt/combat/idle", "wander": "bt/passive/wander"}}}},
    {"name": "villager_guard",    "config": {"template_ref": "warrior_base", "fields": {"aggression": "neutral", "attack_power": 15.0, "defense": 20.0, "is_boss": false, "loot_table": "", "max_hp": 200, "move_speed": 3.0, "perception_range": 25.0}, "behavior": {"fsm_ref": "fsm_combat_basic", "bt_refs": {"attack": "bt/combat/attack", "chase": "bt/combat/chase", "idle": "bt/combat/idle", "patrol": "bt/combat/patrol"}}}},
    {"name": "guard_basic",       "config": {"template_ref": "tpl_guard",    "fields": {"hp": 100}, "behavior": {"fsm_ref": "guard", "bt_refs": {"patrol": "guard/patrol"}}}}
  ]
}
```

### 分歧点

- ✅ **契约形状全部对齐**：`{template_ref, fields, behavior: {fsm_ref, bt_refs}}` 6 条都符合服务端 `ADMINTemplate` 结构
- ✅ **`template_ref` 语义**：ADMIN 指"字段集合模板"（warrior_base / ranger_base / passive_npc / tpl_guard），非游戏角色 archetype。服务端 [admin_template.go:23](../../internal/runtime/npc/admin_template.go#L23) 将其作为不透明字符串忽略，语义差异不引发冲突
- ✅ **`perception_range` 单字段**：PR #18 fallback chain 已覆盖，visual/auditory 自动回落到此字段
- ⚠️ **`guard_basic.fields` 只有 `hp: 100`，无 `max_hp`**：T9 建字段时没看到已有 `max_hp` 造成重复，数据噪声。服务端把两者都通过 `SetDynamic` 写入 BB，互不影响；但若后续治理统一用 `max_hp`，此实例要改
- `loot_table`、`aggression`、`defense`、`is_boss` 等为运营扩展字段：服务端 `SetDynamic` 动态注册写 BB，未被任何 BT 节点消费，透明保留

---

## 汇总：能端到端跑通吗？

**不能**，除非 `docs/specs/admin-data-cleanup/`（ADMIN 侧治理）完成以下至少一项：

| # | 问题 | 归属 | 阻断级别 |
|---|------|-----|---------|
| 1 | BT 节点格式统一（`action` → `params.name`、扁平 check_bb_float → `params` 包裹、去 `target_key`、补空节点） | ADMIN | 🔴 阻断 5 / 6 NPC 启动 |
| 2 | event_types 在 config 内补 `name` 字段 | ADMIN（或服务端改用外层 name） | 🟡 导致 3 事件塌缩 1 条 |
| 3 | FSM 引入运行时 BB key（`current_hp` / `target_distance` 等）替代静态 field | ADMIN 架构升级 | 🟡 FSM 行为不合理但能跑 |
| 4 | `guard_basic` 用 `max_hp` 替代 `hp` | ADMIN 数据清理 | 🟢 零阻断 |

**当前可跑通的最小数据集**：仅 `guard_basic` 一个 NPC。`main.go spawnFromADMINTemplates` 会在其他 5 个遇到 BT parse error 时 `continue` 跳过（[main.go:226-230](../../cmd/server/main.go#L226-L230)），最终 Registry 里只有 1 个 NPC。

---

## 本轮交付物边界

- ✅ PR #17 完成 `{template_ref, fields, behavior}` 解析路径 + HTTPSource 端点对齐
- ✅ PR #18 完成 `perception_range` 合并字段 fallback
- 🚫 **不接 ADMIN 侧 BT / FSM / event config 脏数据**：服务端保持严格契约执行
- 🚫 **不作 Go 测试回归 fixture**：fixture 以契约正确形态为锚

ADMIN 侧治理完成后，再做一轮干净数据的真实 e2e（`docker compose up --build` + 服务端日志观察 spawn 成败）。
