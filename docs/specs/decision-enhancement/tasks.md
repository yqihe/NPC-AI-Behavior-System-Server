# decision-enhancement 任务拆解

## T1: BB Key 新增 + DecisionInput + DecisionWeights 类型定义 (R3, R11) [x]

**文件**：
- `internal/core/blackboard/keys.go`
- `internal/runtime/decision/decision.go`

**做完了是什么样**：
- keys.go 新增 4 个 Key：`decision_winner`(string)、`threat_score`(float64)、`need_score`(float64)、`emotion_score`(float64)
- decision.go 新增 `DecisionInput` struct（Perceived/NeedUrgency/EmotionValue/Weights）
- decision.go 新增 `DecisionWeights` struct（Threat/Needs/Emotion）+ `DefaultWeights` 变量
- 编译通过，不改 Evaluate 签名（下一步改）

---

## T2: Evaluate 重构为多维评分 (R1, R2, R4, R5, R6, R7, R8, R9) [x]

**文件**：
- `internal/runtime/decision/decision.go`
- `internal/runtime/decision/decision_test.go`

**做完了是什么样**：
- Evaluate 签名改为 `(bb, npcPos, input DecisionInput, evtTypes, dt)`
- 威胁维度：取 perceived 最高 Strength，写 threat_level/threat_source/last_event_type（v2 兼容）
- 需求维度：直接使用 input.NeedUrgency
- 情绪维度：直接使用 input.EmotionValue
- 三维原始分写 BB（threat_score/need_score/emotion_score）
- 加权仲裁写 decision_winner
- 无事件时威胁衰减不变
- 测试覆盖：纯威胁/需求优先/情绪优先/默认权重/三维分写入/衰减

---

## T3: Scheduler 组装 DecisionInput (R12, R13) [x]

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- 组件化路径：从 personality 组件读权重（无则用 DefaultWeights），从 BB 读 need_lowest_val + NeedsComponent 计算 urgency，从 BB 读 emotion_dominant_val
- 组装 DecisionInput 传给 Evaluate
- v2 兼容路径：DecisionInput{Perceived, Weights: DefaultWeights}
- 新增辅助函数 `calcNeedUrgency(bb, needs) float64`

---

## T4: 现有测试适配 (R10, R17) [x]

**文件**：
- `internal/runtime/integration_test.go`
- `internal/runtime/verify_attack_test.go`
- `internal/runtime/component_integration_test.go`

**做完了是什么样**：
- 所有 `center.Evaluate(bb, pos, perceived, evtTypes, dt)` 调用改为 `center.Evaluate(bb, pos, decision.DecisionInput{Perceived: perceived, Weights: decision.DefaultWeights}, evtTypes, dt)`
- 现有 6 个集成场景全部通过
- e2e 测试全部通过

---

## T5: 多维决策集成测试 (R14, R15, R16) [x]

**文件**：
- `internal/runtime/decision_integration_test.go`

**做完了是什么样**：
- 测试 1 需求优先：NPC 有 needs（hunger urgency=80），无威胁 → decision_winner="needs"
- 测试 2 情绪优先：timid NPC（emotion weight=0.5），高恐惧 + 低威胁 → decision_winner="emotion"
- 测试 3 威胁压制：高威胁事件到达 → decision_winner="threat"
- 测试 4 默认权重：无 personality → decision_winner 始终 "threat"

---

## 执行顺序

```
T1  类型定义 + BB Key
 └→ T2  Evaluate 重构
     └→ T3  Scheduler 组装
         └→ T4  现有测试适配
             └→ T5  多维决策集成测试
```

严格顺序。
