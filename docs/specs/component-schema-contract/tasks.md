# component-schema-contract 任务拆解

## T1: 组件 Schema — identity + position + behavior (R1, R2, R5)

**文件**：
- `configs/schemas/components/identity.json`
- `configs/schemas/components/position.json`
- `configs/schemas/components/behavior.json`

**做完了是什么样**：3 个文件，每个包含 `component` + `display_name` + `blackboard_keys` + `schema` 信封。identity 的 `blackboard_keys` 为 `["npc_type"]`，position 为 `["npc_pos_x", "npc_pos_z"]`，behavior 为 `["fsm_state", "current_action", "threat_level", "threat_source", "threat_expire_at", "last_event_type", "current_time"]`。所有 Key 名与 `keys.go` 一致。

---

## T2: 组件 Schema — perception + movement (R1, R2, R3, R4, R5)

**文件**：
- `configs/schemas/components/perception.json`
- `configs/schemas/components/movement.json`

**做完了是什么样**：perception 含 `attention_capacity` 字段（integer, ≥1, 默认 5）。movement 含 `move_type` 枚举（wander/patrol/follow）+ `if/then` 条件依赖（wander → wander_radius 必填，patrol → patrol_waypoints 必填）。movement 的 `blackboard_keys` 为 `["move_state", "npc_pos_x", "npc_pos_z"]`。

---

## T3: 组件 Schema — personality (R1, R2, R3, R4)

**文件**：
- `configs/schemas/components/personality.json`

**做完了是什么样**：含 `personality_type` 枚举（timid/aggressive/docile/curious）+ `decision_weights` 对象（threat/needs/emotion 三项，均 0-1）+ `if/then` 条件依赖（aggressive → aggro_range 必填，timid → flee_threshold 必填）。`blackboard_keys` 为空数组。

---

## T4: 组件 Schema — needs + emotion (R1, R2, R5)

**文件**：
- `configs/schemas/components/needs.json`
- `configs/schemas/components/emotion.json`

**做完了是什么样**：needs 含 `need_types` 数组（每项 name/current/max/decay_rate），`blackboard_keys` 为 `["need_lowest", "need_lowest_val"]`。emotion 含 `emotion_states` 数组（每项 name/value/accumulate_rate/decay_rate），`blackboard_keys` 为 `["emotion_dominant", "emotion_dominant_val"]`。两个数组都有 `minItems: 1`。

---

## T5: 组件 Schema — memory + social (R1, R2, R3, R5)

**文件**：
- `configs/schemas/components/memory.json`
- `configs/schemas/components/social.json`

**做完了是什么样**：memory 含 `capacity`(integer ≥1) + `memory_types`(string[] minItems:1) + `decay_time`(number)，`blackboard_keys` 为 `["memory_count"]`。social 含 `group_id` + `faction` + `role`(enum: leader/follower) + `follow_target`，`blackboard_keys` 为 `["group_id", "social_role"]`。social 全部字段非必填。

---

## T6: 预设定义 (R6, R7, R8)

**文件**：
- `configs/schemas/presets/simple.json`
- `configs/schemas/presets/reactive.json`
- `configs/schemas/presets/autonomous.json`
- `configs/schemas/presets/social.json`

**做完了是什么样**：4 个文件，每个含 `preset` + `display_name` + `description` + `required_components` + `default_components` + `optional_components`。所有预设 `required_components` 包含 identity 和 position。4 个预设按复杂度递增，每级在前一级基础上把 optional 的组件移到 default。social 预设的 `optional_components` 为空数组。

---

## T7: BT 节点类型 Schema — composite + decorator (R9, R10)

**文件**：
- `configs/schemas/node_types/sequence.json`
- `configs/schemas/node_types/selector.json`
- `configs/schemas/node_types/parallel.json`
- `configs/schemas/node_types/inverter.json`

**做完了是什么样**：sequence/selector/inverter 的 `params_schema` 为空对象 `{"type": "object"}`（无参数）。parallel 的 `params_schema` 含 `policy` 字段（enum: require_all/require_one，默认 require_all）。category 分别为 composite/composite/composite/decorator。

---

## T8: BT 节点类型 Schema — leaf (R9, R10, R11)

**文件**：
- `configs/schemas/node_types/check_bb_float.json`
- `configs/schemas/node_types/check_bb_string.json`
- `configs/schemas/node_types/set_bb_value.json`
- `configs/schemas/node_types/stub_action.json`

**做完了是什么样**：check_bb_float 的 op 枚举为 6 种（== != > >= < <=），value 为 number。check_bb_string 的 op 枚举为 2 种（== !=），value 为 string。set_bb_value 的 value 无类型限制。stub_action 的 result 枚举为 success/failure/running，默认 success。全部 category 为 leaf。参数与 Go factory 精确匹配。

---

## T9: FSM 条件类型 Schema (R12, R13, R14)

**文件**：
- `configs/schemas/condition_types/leaf.json`
- `configs/schemas/condition_types/composite.json`

**做完了是什么样**：leaf 含 key(必填) + op(必填，7 种枚举) + value(any) + ref_key(string)，注明 value 和 ref_key 二选一。composite 含 and(array) + or(array)，子项递归引用条件结构。与 `rule.Condition` 精确一致。

---

## T10: 区域数据结构 Schema (R15, R16)

**文件**：
- `configs/schemas/region.json`

**做完了是什么样**：含 region_id(必填) + name(必填) + region_type(enum: wilderness/town/dungeon) + boundary(polygon points 数组) + weather(default + cycle) + spawn_table(数组，每项 template_ref/count/spawn_points/wander_radius/respawn_seconds) + properties(扩展对象)。

---

## T11: Schema 校验测试 (R17, R18)

**文件**：
- `configs/schemas/schemas_test.go`

**做完了是什么样**：`go test ./configs/schemas/...` 通过。测试内容：
1. 遍历所有 JSON 文件，`json.Valid()` 通过
2. 组件文件 Unmarshal 后 `component` + `display_name` + `blackboard_keys` + `schema` 字段非空
3. 预设文件 `required_components` 包含 identity 和 position
4. 节点类型文件 `node_type` 值在 `bt.DefaultRegistry()` 注册名中
5. 组件 `blackboard_keys` 中属于已有 Key 的名称与 `keys.go` 注册表一致
6. 8 个新增 Key 名在全部 Schema 文件间无重复定义

---

## 执行顺序

```
T1 ─┐
T2 ─┤
T3 ─┤ 无依赖，可并行
T4 ─┤
T5 ─┘
T6 ──── 依赖 T1-T5（预设引用组件名）
T7 ─┐
T8 ─┘── 无依赖，可并行
T9 ──── 无依赖
T10 ─── 无依赖
T11 ─── 依赖全部（校验所有文件）
```
