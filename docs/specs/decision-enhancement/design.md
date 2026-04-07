# decision-enhancement 设计方案

## 方案描述

### 1. Evaluate 新签名

```go
// DecisionInput 决策中心的输入，由 Scheduler 从组件数据组装
type DecisionInput struct {
    Perceived    []perception.PerceiveResult // 感知结果（威胁维度）
    NeedUrgency  float64                     // 需求紧迫度 0~100（需求维度）
    EmotionValue float64                     // 主导情绪强度（情绪维度）
    Weights      DecisionWeights             // 性格权重
}

// DecisionWeights 三维决策权重
type DecisionWeights struct {
    Threat  float64
    Needs   float64
    Emotion float64
}

// DefaultWeights v2 兼容默认权重（纯威胁决策）
var DefaultWeights = DecisionWeights{Threat: 1.0, Needs: 0, Emotion: 0}

func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos event.Vec3,
    input DecisionInput, evtTypes map[string]*event.EventTypeConfig, dt float64)
```

### 2. 多维评分逻辑

```go
func (c *Center) Evaluate(...) {
    // 1. 威胁维度：取 perceived 最高 Strength（与 v2 一致）
    threatScore, maxEvent := c.calcThreatScore(input.Perceived)

    // 2. 需求维度：直接使用传入的 urgency
    needScore := input.NeedUrgency

    // 3. 情绪维度：直接使用传入的 emotion value
    emotionScore := input.EmotionValue

    // 4. 写三维原始分到 BB
    blackboard.Set(bb, blackboard.KeyThreatScore, threatScore)
    blackboard.Set(bb, blackboard.KeyNeedScore, needScore)
    blackboard.Set(bb, blackboard.KeyEmotionScore, emotionScore)

    // 5. 加权仲裁
    w := input.Weights
    weightedThreat  := threatScore * w.Threat
    weightedNeed    := needScore * w.Needs
    weightedEmotion := emotionScore * w.Emotion

    winner := "threat"
    maxWeighted := weightedThreat
    if weightedNeed > maxWeighted {
        winner = "needs"
        maxWeighted = weightedNeed
    }
    if weightedEmotion > maxWeighted {
        winner = "emotion"
    }

    blackboard.Set(bb, blackboard.KeyDecisionWinner, winner)

    // 6. 威胁维度的 BB 写入（保持 v2 兼容）
    if maxEvent != nil {
        // 写 threat_level/threat_source/last_event_type/threat_expire_at
    } else if len(input.Perceived) == 0 {
        c.decay(bb, dt)
    }
}
```

**关键设计**：威胁维度的 BB 写入（threat_level 等）不受仲裁结果影响——即使 `decision_winner = "needs"`，threat_level 仍然正确反映当前威胁等级。FSM 可以同时读 `decision_winner` 和 `threat_level` 做转换条件。

### 3. 需求紧迫度计算

Scheduler 从 BB 读取 `need_lowest_val` 和对应的 NeedsComponent 计算 urgency：

```go
// 在 Scheduler 中组装
needUrgency := 0.0
if needs, ok := npc.GetComponent[*component.NeedsComponent](inst, "needs"); ok {
    lowestVal, _ := blackboard.Get(inst.BB, blackboard.KeyNeedLowestVal)
    // 找到对应 need 的 max 值
    lowestName, _ := blackboard.Get(inst.BB, blackboard.KeyNeedLowest)
    for _, n := range needs.NeedTypes {
        if n.Name == lowestName {
            needUrgency = (n.Max - lowestVal) / n.Max * 100
            break
        }
    }
}
```

urgency = `(max - current) / max × 100`：current=0 → urgency=100（最紧迫），current=max → urgency=0（完全满足）。

### 4. Scheduler 组装 DecisionInput

```go
// 在 Scheduler Tick 的 AI 管线中
if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
    input := decision.DecisionInput{
        Perceived: perceived,
        Weights:   decision.DefaultWeights,
    }

    // 读取 personality 权重
    if pers, ok := npc.GetComponent[*component.PersonalityComponent](inst, "personality"); ok {
        input.Weights = decision.DecisionWeights{
            Threat:  pers.DecisionWeights.Threat,
            Needs:   pers.DecisionWeights.Needs,
            Emotion: pers.DecisionWeights.Emotion,
        }
    }

    // 读取需求紧迫度
    if needs, ok := npc.GetComponent[*component.NeedsComponent](inst, "needs"); ok {
        input.NeedUrgency = calcNeedUrgency(inst.BB, needs)
    }

    // 读取情绪强度
    emotionVal, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionDominantVal)
    input.EmotionValue = emotionVal

    s.Decision.Evaluate(inst.BB, inst.Position, input, s.EvtTypes, dt)
    // ... FSM + BT Tick
}
```

### 5. BB Key 新增

```go
var KeyDecisionWinner = NewKey[string]("decision_winner")     // "threat" / "needs" / "emotion"
var KeyThreatScore    = NewKey[float64]("threat_score")        // 威胁原始分
var KeyNeedScore      = NewKey[float64]("need_score")          // 需求原始分
var KeyEmotionScore   = NewKey[float64]("emotion_score")       // 情绪原始分
```

### 6. v2 兼容路径

v2 兼容路径（`inst.FSM != nil` 分支）：构造 `DecisionInput{Perceived: perceived, Weights: DefaultWeights}`，需求分和情绪分为 0 → `decision_winner` 始终为 `"threat"` → 行为与 v2 完全一致。

---

## 方案对比

### 备选方案：决策中心直接读组件（不选）

Decision.Evaluate 接收 NPC Instance，自己从组件提取 personality/needs/emotion 数据。

```go
func (c *Center) Evaluate(bb *blackboard.Blackboard, inst *npc.Instance, ...) {
    pers, _ := npc.GetComponent[*component.PersonalityComponent](inst, "personality")
    needs, _ := npc.GetComponent[*component.NeedsComponent](inst, "needs")
    // ...
}
```

**不选的理由**：
1. 违反依赖方向——`decision/` 包需要 import `npc/` 和 `component/`，形成 `decision/ → npc/ → component/` 依赖链，而 `npc/` 也依赖 `decision/`（间接通过 Scheduler），有循环风险
2. Decision 变成上帝对象——知道所有组件的细节，职责不清
3. 测试困难——需要构造完整 Instance 而不是简单的 DecisionInput struct

选定方案中 Scheduler 负责组装 DecisionInput（Scheduler 已经知道所有组件），Decision 只做评分仲裁。职责清晰。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 参数 | **不违反** | 权重从 personality 组件配置读取 |
| 禁止 BT 反向驱动 FSM | **不违反** | 决策结果写 BB，FSM 读 BB 做转换，方向不变 |
| 禁止 core/ import runtime/ | **不违反** | decision 在 runtime/ 下 |
| 禁止 Blackboard 裸 map | **不违反** | 新 Key 通过 BBKey[T] 注册 |
| 禁止 Key 散落各文件 | **不违反** | 4 个新 Key 在 keys.go |
| 禁止静默降级 | **不违反** | 无 personality → 用 DefaultWeights 且日志记录 |
| 禁止过度设计 | **不违反** | DecisionInput 是 plain struct，无接口无抽象 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | 中性 | 新事件自动参与威胁维度 |
| 加 NPC 类型 | **正面** | 不同 personality 权重产生不同决策行为 |
| NPC 间交互 | **正面** | 仲裁框架可扩展——未来加第四维度（社交压力）只需扩展 DecisionInput |

---

## 依赖方向

```
internal/runtime/ (Scheduler)
  → internal/runtime/decision/     (Evaluate + DecisionInput)
  → internal/runtime/component/    (PersonalityComponent, NeedsComponent)
  → internal/runtime/perception/   (PerceiveResult)

internal/runtime/decision/
  → internal/runtime/perception/   (PerceiveResult，不变)
  → internal/core/blackboard/      (不变)
  → internal/runtime/event/        (不变)
```

无新增依赖方向。decision/ 不 import component/ 或 npc/（通过 DecisionInput 解耦）。

---

## 并发安全

无新增共享状态。DecisionInput 是每 NPC 每 Tick 的栈变量。新 BB Key 的读写在同一 NPC 的 Tick 内顺序执行。

---

## 配置变更

无新增配置文件。personality 组件的 decision_weights 已在需求 0 Schema 和需求 1 struct 中定义。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `decision/` | 纯威胁输入（needs=0, emotion=0）→ winner="threat" |
| `decision/` | 需求优先场景（低威胁 + 高 urgency + needs 权重大）→ winner="needs" |
| `decision/` | 情绪优先场景（timid 权重 + 高恐惧）→ winner="emotion" |
| `decision/` | 默认权重 → 始终 winner="threat" |
| `decision/` | 三维原始分正确写入 BB |
| `decision/` | 威胁衰减逻辑不变 |

### 集成测试

| 场景 | 验证 |
|------|------|
| NPC 有 needs 组件且饥饿，无威胁 | decision_winner="needs" |
| NPC 有 emotion 组件且恐惧高 + timid 性格，低威胁 | decision_winner="emotion" |
| 高威胁事件压制需求和情绪 | decision_winner="threat" |
| v2 旧格式 NPC | 现有 6 个场景通过 |
