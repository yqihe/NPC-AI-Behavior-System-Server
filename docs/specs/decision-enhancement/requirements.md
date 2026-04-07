# decision-enhancement 需求分析

## 动机

v2 的决策中心只有一个维度——威胁。NPC 的行为完全由外部事件驱动：有威胁就跑，没威胁就闲。这导致：

1. **NPC 没有自主行为**：没有事件时 NPC 永远 Idle。真实的 AI 角色应该在没有威胁时也有事做——饿了找食物，累了休息，好奇时探索。
2. **性格组件是死配置**：`PersonalityComponent.DecisionWeights` 在需求 1 已定义（threat/needs/emotion 三项权重），但决策中心不读它——所有 NPC 的决策逻辑完全相同。
3. **情绪不影响行为**：`EmotionComponent` 每 Tick 写 BB（emotion_dominant/emotion_dominant_val），但决策中心不读——恐惧累积到 100 和恐惧为 0 的 NPC 行为一样。

需求组件和情绪组件的 Tick 逻辑在需求 1 已实现（衰减 + 写 BB），但产出的数据没有消费者。决策中心是唯一的消费者。

## 优先级

**高**。依赖需求 1（组件化）和需求 2（感知强度），被需求 4（记忆系统）依赖——记忆影响决策需要多维决策框架先就位。

## 预期效果

### 场景 1：多维评分仲裁

NPC 有 personality `decision_weights: {threat: 0.5, needs: 0.3, emotion: 0.2}`。

当前状态：
- 威胁分 = 30（远处的小事件）
- 需求分 = 70（hunger 的 current 很低，urgency 高）
- 情绪分 = 20（轻微恐惧）

加权总分：
- 威胁得分 = 30 × 0.5 = 15
- 需求得分 = 70 × 0.3 = 21 ← **最高**
- 情绪得分 = 20 × 0.2 = 4

决策结果：**需求优先**（hunger urgency 驱动行为），写 BB `decision_winner = "needs"`。

FSM 可通过 `decision_winner` 条件转换到"觅食"状态。

### 场景 2：高威胁压制需求和情绪

同一个 NPC，突然爆炸 severity=80 在附近：
- 威胁分 = 75
- 需求分 = 70
- 情绪分 = 50（恐惧因威胁累积）

加权总分：
- 威胁得分 = 75 × 0.5 = 37.5 ← **最高**
- 需求得分 = 70 × 0.3 = 21
- 情绪得分 = 50 × 0.2 = 10

决策结果：**威胁优先**（回到逃跑行为），`decision_winner = "threat"`。

### 场景 3：胆小 NPC vs 好战 NPC 面对同一威胁

威胁分 = 40，需求分 = 50，情绪分 = 60（恐惧）。

**胆小 NPC**（timid）weights: `{threat: 0.3, needs: 0.2, emotion: 0.5}`：
- 情绪得分 = 60 × 0.5 = 30 ← 最高 → `decision_winner = "emotion"` → 情绪驱动（逃跑）

**好战 NPC**（aggressive）weights: `{threat: 0.7, needs: 0.2, emotion: 0.1}`：
- 威胁得分 = 40 × 0.7 = 28 ← 最高 → `decision_winner = "threat"` → 威胁驱动（战斗）

同一环境，不同性格产生不同行为。

### 场景 4：无 personality 组件的 NPC 保持 v2 行为

没有 personality 组件的 NPC（如 v2 旧格式转换的），使用默认权重 `{threat: 1.0, needs: 0, emotion: 0}`——等价于 v2 的纯威胁决策。

## 依赖分析

- **依赖**：
  - 需求 1 组件化架构（已完成）：NeedsComponent/EmotionComponent/PersonalityComponent 数据结构 + BB Key
  - 需求 2 感知深化（已完成）：PerceiveResult 传递感知强度

- **被依赖**：
  - 需求 4 记忆系统：记忆影响决策需要多维框架
  - 需求 6 社交系统：群组决策需要仲裁器

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/decision/` | **重构** | 2 | Evaluate 升级为多维评分 + 仲裁 |
| `internal/runtime/scheduler.go` | **修改** | 1 | 传入 personality/needs/emotion 组件数据 |
| `internal/core/blackboard/keys.go` | **修改** | 1 | 新增 decision_winner/decision_scores BB Key |
| 测试文件 | **新增+修改** | 3-4 | 多维决策单元测试、集成测试 |

预估 8-10 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **中性** | 新事件自动参与威胁维度评分 |
| 加 NPC 类型 | **正面** | 不同 personality_type 的 NPC 对同一环境产生不同行为 |
| NPC 间交互 | **间接** | 多维仲裁器是群组决策的基础 |

## 验收标准

### 多维评分

- **R1**：决策中心从 BB 读取 `need_lowest_val`（需求紧迫度）和 `emotion_dominant_val`（主导情绪强度），与感知强度（威胁分）一起参与评分
- **R2**：需求紧迫度计算：`urgency = (max - current) / max × 100`，即 current 越低 urgency 越高（0~100 归一化）
- **R3**：三个维度的原始分（threat_score / need_score / emotion_score）写入 BB，供调试查看

### 加权仲裁

- **R4**：从 NPC 的 personality 组件读取 `decision_weights`（threat/needs/emotion），计算加权得分
- **R5**：加权最高的维度胜出，写 BB `decision_winner`（值为 `"threat"` / `"needs"` / `"emotion"`）
- **R6**：无 personality 组件时使用默认权重 `{threat: 1.0, needs: 0, emotion: 0}`（等价 v2 行为）
- **R7**：无 needs 组件时需求分 = 0；无 emotion 组件时情绪分 = 0

### 威胁维度保持兼容

- **R8**：威胁分仍取 perceived 中最高 Strength，写 BB `threat_level` / `threat_source` / `last_event_type`（与 v2 一致）
- **R9**：威胁衰减逻辑不变（无事件时按 DecayRate 衰减）
- **R10**：现有 6 个集成测试场景全部通过

### BB Key 新增

- **R11**：新增 BB Key：`decision_winner`(string)、`threat_score`(float64)、`need_score`(float64)、`emotion_score`(float64)

### Scheduler 适配

- **R12**：Scheduler 从 NPC 组件提取 personality weights，传给 Decision.Evaluate
- **R13**：Scheduler 从 BB 读取 need_lowest_val 和 emotion_dominant_val 传给 Decision.Evaluate（这些值由组件 Tick 在上一帧写入）

### 测试

- **R14**：多维评分单元测试：三个维度不同权重，验证 decision_winner 正确
- **R15**：默认权重测试：无 personality 时 decision_winner 始终为 "threat"
- **R16**：集成测试：NPC 有 needs 组件且饥饿时，decision_winner 在无威胁时为 "needs"
- **R17**：e2e 测试全部通过

## 不做什么

- **不做 FSM 条件适配**：FSM 配置中加 `decision_winner` 条件是配置层的事，运营通过 ADMIN 添加，不是决策中心的代码改动
- **不做需求驱动的行为**：决策中心只输出 `decision_winner = "needs"`，具体"觅食"行为的 BT 节点在需求 5（移动系统）
- **不做情绪累积逻辑**：情绪的 AccumulateRate 在威胁到来时怎么触发累积，在需求 4（记忆系统）中与记忆联动
- **不做记忆影响**：记忆修正决策阈值在需求 4
