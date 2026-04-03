# 未来规划

## 当前状态

Core / Runtime / Gateway / Experiment / Config 五层全部完成。扩展轴步骤 1-4 验证通过。Unity 客户端对接中。

---

## 短期（答辩前）

### 1. Unity 客户端联调

- **状态**：进行中（Client 端独立仓库开发）
- **内容**：WS 通信层 + NPC 可视化 + GM 面板 + AutoTestRunner
- **服务端配合**：协议已定义（`docs/protocol.md`），预计无需改动

### 2. 扩展轴步骤 5：NPC 间交互

- **状态**：待规划
- **内容**：警察保护平民、NPC 间信息传递
- **可能的改动**：决策中心扩展（当前只处理事件→NPC，需要支持 NPC→NPC 交互），可能需要新的 BB Key 和 BT 节点
- **风险**：可能需要改 runtime 层代码，需要评估是否违反"加配置不改代码"原则

### 3. 论文数据采集

- **状态**：待执行
- **内容**：运行 experiment 层的定性/定量测试，生成图 1-7 + 表 1 数据
- **命令**：见 `docs/specs/experiment-layer/data-collection-guide.md`
- **前置**：无，可以现在就跑

### 4. 答辩演示准备

- **状态**：待规划
- **内容**：
  - Docker 一键启动演示环境
  - Unity 客户端连接 → spawn NPC → 触发事件 → 观察行为
  - GM 面板实时调试
  - 对照实验数据展示

---

## 中期（答辩后优化）

### 5. 更多 NPC 类型和事件

- 补充 NPC 类型：商人（Merchant）、医生（Doctor）、守卫（Guard）
- 补充事件类型：robbery（抢劫）、medical_emergency（医疗紧急）、alarm（警报）
- 验证扩展轴在更多类型下的可扩展性

### 6. BT 节点丰富化

- 当前 BT 叶子节点是 stub（占位），返回固定结果
- 实现真实行为节点：move_to（移动到目标）、play_animation（播放动画）、wait（等待）
- 与 Unity 客户端的 NPC 位置同步

### 7. AOI 空间优化

- 当前 world_snapshot 全量广播所有 NPC
- 实现 AOI（Area of Interest）：客户端只收到视野范围内的 NPC
- 实现空间索引：感知过滤从 O(n) 优化为 O(log n)
- 位置在 `internal/runtime/world/`（目录已预留）

### 8. 配置热更新

- 当前 MongoSource 启动时加载一次，运行时不更新
- 实现 MongoDB Change Stream 监听 → 运行时热更新配置
- 无需重启服务即可加载新 NPC 类型/事件类型

---

## 长期（如果继续维护）

### 9. 多节点部署

- 当前单节点，所有 NPC 在一个进程中
- 实现分区（Sharding）：按地图区域分配 NPC 到不同节点
- 需要 Redis 做跨节点状态同步（此时 Redis 有真实使用场景）

### 10. NPC 持久化

- 当前 NPC 状态全在内存，服务重启后丢失
- 实现 NPC 状态快照持久化到 MongoDB
- 服务启动时恢复 NPC 状态

### 11. 性能优化

- Tick 调度分级：Sleep/Mid/High 三级频率（v1 已有设计，可复用）
- 连接池化 MongoDB（如果热更新需要长连接）
- 广播压缩（二进制协议替代 JSON，如 protobuf）

---

## 不做的事情（明确排除）

| 排除项 | 理由 |
|--------|------|
| Redis 缓存 | 单节点内存足够，红线禁止无使用场景提前接入 |
| 认证/鉴权 | 毕设不需要 |
| Admin 管理接口 | GM 面板通过 WS 协议实现，不需要独立 HTTP API |
| 消息压缩 | JSON 明文便于调试，毕设规模下性能不是瓶颈 |
| CI/CD | 手动部署足够 |
