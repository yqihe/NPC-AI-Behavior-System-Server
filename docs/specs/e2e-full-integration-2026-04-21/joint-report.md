# e2e Full Integration — 联调报告 2026-04-21

**范围**：L2 加载级 — ADMIN HTTP 源 5 端点拉取 + 双路径 spawn 收敛 + 故障注入 fail-fast（见 [runbook.md](runbook.md) §验收目标）

**参与方**：Admin（配置 + 对账脚本）× Server-v1（运行时）

**结论**：**三轮全绿**，实现与 spec 契约在 L2 层级对齐，暴露 1 处已知实现落差（记入 §暴露的 Issue）。

> **归档更新（2026-04-21 同日）**：I1 已由 [PR #41](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/41) 对称落地（`fetchNpcTemplatesEndpoint` 解码 45016 details[] → `config.http.npc_templates.dangling` 结构化日志）。R2B "含已知落差" 标记仅保留历史采集语境，当前 main 上已不再有 `status 500` 信息丢失。`未来改进 §1` 同步标记完成。

---

## 三轮结果

### R1 Happy Path — PASS ✓

| 锚点 | 实际 | 预期 |
|------|------|------|
| config.http.loaded × 5（event_types/fsm_configs/bt_trees/npc_templates/regions） | 5/3/6/4/2 | 5/3/6/4/2 |
| config.source type=http | ✓ | ✓ |
| events.loaded count=5 / zones.loaded count=2 | ✓ | ✓ |
| admin_spawn.done spawned=4 template_count=4 | ✓ | ✓ |
| server.start addr=:9820 | ✓ | ✓ |
| npc_active_count（双 zone 标签求和） | 2 + 4 = **6** | 6 |
| npc_tick_total（跑 ~65s） | 654 | ≥ 10 |
| 失败锚点 6 项（config.http_error / regions.dangling / zones.spawn_error / admin_spawn.parse_error / admin_spawn.instance_error / cascade.violations） | 全 0 行 | 全 0 行 |

**disable fan-out** 验证：Admin seed 5 条 NPC 模板含 1 条 `enabled=false`，`/api/configs/npc_templates` 返回 4，Server 侧 admin_spawn 按 4 模板实例化，总 npc_active_count = zone spawn 2（e2e_village）+ template spawn 4（global） = 6，完全匹配 §1.3 R15 双路径公式。

### R2A Dangling Region — PASS ✓

**注入**：`e2e_village.spawn_table[0].template_ref = missing_npc_xxx`

| 锚点 | 实际 | 预期 |
|------|------|------|
| 前 4 端点 loaded（event_types/fsm_configs/bt_trees/npc_templates） | 5/3/6/4 | 5/3/6/4 |
| config.http.regions.dangling | region_id=e2e_village / ref_type=npc_template_ref / ref_value=missing_npc_xxx / reason=missing_or_disabled | 1 行 |
| config.http_error err | `code=47011, count=1` | 1 行 |
| regions loaded / zones.loaded / admin_spawn.done / server.start | 全 0 行 | 全 0 行 |
| RestartCount | 8 | ≥ 2 |

**PR #37 的 47011 fail-fast 路径端到端联调通过**，details 解码出的字段（region_id / ref_type / ref_value / reason）全部落入结构化日志。

### R2B Dangling fsm_ref — PASS（含已知落差） ✓

**注入**：`e2e_full.behavior.fsm_ref = missing_fsm_xxx`

| 锚点 | 实际 | 预期 |
|------|------|------|
| 前 3 端点 loaded（event_types/fsm_configs/bt_trees） | 5/3/6 | 5/3/6 |
| config.http_error err | `api/configs/npc_templates: status 500`（无业务码、无 ref_value） | 1 行 status 500 |
| npc_templates loaded / regions loaded / zones.loaded / admin_spawn.done / server.start | 全 0 行 | 全 0 行 |
| config.http.npc_templates.dangling | 0 行 | 0 行（未实现，spec §2B.9 未来锚点占位） |
| RestartCount | 12 | ≥ 2 |

---

## 暴露的 Issue

### I1（中）generic fetchEndpoint 不解码业务错误 body — ✅ 已解决（PR #41）

**现象**：`internal/config/http_source.go` 中通用 `fetchEndpoint`（event_types/fsm_configs/bt_trees/npc_templates 共用路径）在 HTTP 非 200 时仅返 `"status <code>"`，不读 body、不解 `code=45016` / `details[]`。联调 R2B 实际观察：45016 暴露给 Admin 的 `ref_value=missing_fsm_xxx / ref_type=fsm_ref` 全部丢失，Server 日志仅剩 `status 500`。

**影响**：故障自解释性缺失 — 仅靠 Server 日志无法定位"哪个模板哪个字段引用了 disabled/missing FSM"，必须回查 Admin 后端 or DB。

**对称方案**：参照 [PR #37](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/37)（regions 47011 解码）扩展至 4 端点，新增 `config.http.<endpoint>.dangling` 结构化日志 + `config.http_error err` 内嵌 `code / count / details` 汇总。

**落地（2026-04-21 同日）**：[PR #41](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/41) 为 npc_templates 端点新增 `fetchNpcTemplatesEndpoint`，500+45016 解码 details[]（复用 Admin `NPCExportDanglingRef` 类型，`npc_name` 字面承载 NPC 名，`ref_type ∈ {fsm_ref, bt_ref}`，`state` 仅 bt_ref 填充）后 fail-fast，对称 PR #37 regions 路径。event_types/fsm_configs/bt_trees 三端点当前仅 status 级落差，非本轮验收锚点。

### I2（低）spec 正则硬编码 fixture 值，与 Admin verify.sh 实际注入值轻微偏离 — ✅ 已解决

**现象**：`expected-log-patterns.md` §2A.2 正则写死 `ref_value=missing_template_xxx`；Admin verify.sh 实际注入 `missing_npc_xxx`。对账 PASS 依赖 Admin 脚本本地口径，但 spec md 自述不一致。

**解法**：spec md 参数化——`ref_value=\S+` 替代硬编码 fixture 值，或把 fixture 值作为 Admin/Server 双边共享的变量表单独抽出。**本 PR 内小修**。

**落地**：正则早在 PR #40 已改为 `\S+`（expected-log-patterns.md §2A.2/§2B.2）；expected-log-patterns.md §故障注入共享 fixture 表集中托管注入值；runbook/patterns 的叙述性 `missing_template_xxx` 同步改为 `missing_npc_xxx`（PR #46）。

### I3（低）启动日志顺序与 runbook §观测锚点清单叙述偏离 — ✅ 已解决

**现象**：runbook 第 57-65 行按"config.loaded → config.source → config.http.loaded × 5 → events.loaded → ..." 叙述；实现里 `config.source` 出现在 5 端点 `config.http.loaded` **之后**（因为 HTTPSource 构造完后才打源标注）。

**影响**：对账用正则存在性+捕获组判定，不按行序卡 PASS/FAIL，**不影响本轮结果**。但 runbook 叙述应对齐实现，避免误导未来读者。

**解法**：runbook §"启动阶段顺序（happy path）" 表格 reorder，把 config.source 挪到第 7 位。**本 PR 内小修**。

**落地**：runbook §"启动阶段顺序（happy path）" 表格已把 `config.http.loaded × 5` 放在 #2、`config.source` 放在 #3（附注"位置在 5 端点之后"），叙述与实现一致。PR #40 合入前已修正。

---

## 性能观察

非本轮验收目标，顺手记录：

| 指标 | 观察值 | 备注 |
|------|--------|------|
| 启动到 tick 稳定耗时 | < 1s（config.loaded → admin_spawn.done 约 23ms，加 server.start 约 24ms 总） | HTTPSource 5 端点串行拉取，总 ~22ms（localhost/host.docker.internal） |
| Tick 频率 | npc_tick_total 654 / ~65s ≈ 10 Hz | 与 tick_rate_ms=100 一致，无漂移 |
| fail-fast 到容器重启完成 | 秒级（Docker restart policy 指数退避，8–10s 到达 restart=8） | 无挂起，Exit(1) 及时响应 |
| 容器内存 | 未专项采集，压测基线参 [test/benchmark/baseline_2026-04-20_script_fix.md](../../../test/benchmark/baseline_2026-04-20_script_fix.md) | — |

---

## 未来改进（独立 PR）

1. ~~**fetchEndpoint 45016 对称解码**（I1）~~ — ✅ 已在 [PR #41](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/41) 落地（npc_templates 端点对称 regions 路径）
2. ~~**spec md I2/I3 小修**~~ — ✅ I3 在 PR #40 合入前已 reorder；I2 叙述性 fixture 值统一于本次归档（详见各 Issue §落地）
3. **L3 运行级 e2e**（FSM 状态转换 / BT 节点轨迹 / perception 事件分发）—— 本轮非目标，后续单独立规范

---

## 交叉引用

- [runbook.md](runbook.md) — Server 侧 reset + 启动 + 观测锚点 + 故障注入
- [expected-log-patterns.md](expected-log-patterns.md) — 正则化日志模式表
- Admin 仓 `docs/specs/e2e-full-integration-2026-04-21/joint-report.md` — 独立追踪，结论一致
- [PR #37](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/37) — regions 47011 fail-fast（已 merge）
- [PR #40](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/40) — 本 spec 目录
- [PR #41](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/41) — npc_templates 45016 对称 fail-fast（已 merge，闭环 I1）
