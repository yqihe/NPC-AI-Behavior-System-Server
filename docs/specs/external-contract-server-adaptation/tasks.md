# external-contract-server-adaptation 任务拆解

**对应文档**：[requirements.md](requirements.md) R1–R24 / [design.md](design.md) §1–§13

执行顺序依赖：T1 → T2 → T5 → T6 → T7 → T4 → T3 → T8 → T9 → T10。T5 / T6 可并行推进。v2 源码删除（T8）必须在所有消费方切换完成后。

---

## T1: admin_template 升级为完整翻译层 (R1, R5, R6, R7, R8, R17) [x]

**文件**：
- `internal/runtime/npc/admin_template.go`（主改）
- `internal/runtime/npc/admin_template_test.go`
- `cmd/server/main.go`（spawnFromADMINTemplates 调用点同步签名）

**做完了是什么样**：
- `NewInstanceFromADMIN` 签名扩参 `compReg *component.Registry`，所有调用点同步
- 默认组件层实例化：identity / position / behavior / perception / movement（design.md §1.1）
- opt-in 组件层：读取 `fields.enable_{memory,emotion,needs,personality,social}` 5 个 bool，true 则调用对应 factory
- absent ≡ false 语义：字段缺失时跳过 factory 调用，不报错
- `fields` 中未被 enable_* / perception_* / move_* / aggression 等已知 key 消费的字段 → SetDynamic 透传（R7）
- perception_range fallback 链保留（visual_range / auditory_range / perception_range 三选一，R8）
- 单元测试覆盖：5 默认组件全装 / opt-in 全 absent 无 5 能力组件 / opt-in 混合启用 / 未知 fields SetDynamic 透传 / hp 孤儿字段透传
- `go build ./...` + `go test ./internal/runtime/npc/...` 全绿

---

## T2: npctest 子包建立 (R21) [x]

**文件**：
- `internal/runtime/npc/npctest/helpers.go`（新建）
- `internal/runtime/npc/npctest/doc.go`（新建）
- `internal/runtime/npc/npctest/helpers_test.go`（新建）
- `docs/standards/red-lines.md`（追加一条）

**做完了是什么样**：
- `NewInstanceWithExtras(id, pos, admin, extras, src, btReg, compReg)` 签名与 design.md §5 一致
- 内部先调 T1 升级后的 `NewInstanceFromADMIN` 完成默认 + opt-in 翻译；再按 extras 逐个调用 `compReg.Create(name, raw)` 追加
- 与 opt-in 结果重名时 extras 覆盖（测试显式意图优先）
- tickables 重排保证 memory(0) → needs(1) → emotion(2) → movement(3)
- helpers_test.go 覆盖：空 extras / memory 注入 / emotion+memory 注入 / 与 opt-in 重名覆盖（fixture 设置 enable_memory=true + extras 也传 memory，验证 extras 版本生效）
- red-lines.md 新增："禁止生产代码 import 以 `test` 结尾的 Go 包"
- 空 server 编译 + `go vet ./...` 通过

---

## T5: 测试迁移 wolf_common 系 17 处 (R13, R21)

**文件**：
- `internal/runtime/decision_integration_test.go`（4 处）
- `internal/runtime/memory_integration_test.go`（3 处）
- `internal/runtime/perception_integration_test.go`（3 处）
- `internal/runtime/component_integration_test.go`（2 处）
- `internal/runtime/social_integration_test.go`（1 处）
- `internal/runtime/movement_integration_test.go`（1 处）
- `internal/runtime/benchmark_test.go`（1 处 wolf_common）
- `internal/runtime/social/group_manager_test.go`（2 处 `npc.TemplateConfig{}` 直构造）

**做完了是什么样**：
- 所有上述调用点从 `LoadNPCTemplate("wolf_common") + ParseNPCTemplate + NewInstanceFromTemplate` 迁移到 `npctest.NewInstanceWithExtras(id, pos, ADMINTemplate, extras, src, btReg, compReg)`
- group_manager_test 两处直构造同样走 npctest helper，不新增单独 API（design §5 单门户红线）
- 每处 extras 按原测试目标精确注入所需组件（memory / emotion / needs / personality / social 按原 config.components 精确复刻）
- `go test ./internal/runtime/...` 在 wolf_common.json 不存在情况下仍全绿

---

## T6: 测试迁移 butterfly_01 系 8 处 (R13)

**文件**：
- `internal/runtime/zone_integration_test.go`（3 处）
- `internal/runtime/component_integration_test.go`（2 处 butterfly_01）
- `internal/runtime/movement_integration_test.go`（2 处 butterfly_01）
- `internal/runtime/benchmark_test.go`（1 处 butterfly_01）

**做完了是什么样**：
- 调用路径从 v2 `LoadNPCTemplate + ParseNPCTemplate + NewInstanceFromTemplate` 切换到新路径 `LoadNPCTemplate → ParseADMINTemplate → NewInstanceFromADMIN`（或 npctest helper，视测试是否需要 extras 注入）
- butterfly 作为 simple 级 NPC 典型不注入任何 opt-in 组件，走默认组件层
- T7 完成后 butterfly_01.json 变为 ADMIN shape，此步骤断言需同步更新

---

## T7: configs 迁移 (R9, R10)

**文件**：
- `configs/npc_templates/butterfly_01.json`（rewrite 为 ADMIN shape）
- `configs/npc_templates/wolf_common.json`（删除）
- `configs/npc_types/guard.json`（删除）
- `configs/bt_trees/guard/alert.json`（删除）
- `configs/bt_trees/guard/defend.json`（删除）

**做完了是什么样**：
- butterfly_01.json 按 design.md §4.2 目标结构写入（`{template_ref: "passive_npc", fields: {...}, behavior: {fsm_ref, bt_refs}}`）
- wolf_common.json 物理删除
- `configs/npc_types/civilian.json` 与 `configs/npc_types/police.json` **不动**（实验层 + e2e 扩展轴 2 依赖）
- `configs/fsm/civilian.json` + `configs/fsm/police.json` + `configs/bt_trees/{civilian,police}/` 全部保留
- `go run ./cmd/sync -api http://localhost:9821` 执行后 configs/ diff 仅涉及 ADMIN 真实同步内容，butterfly_01 不被覆盖破坏

---

## T4: 生产路径切换到 NewInstanceFromADMIN (R1)

**文件**：
- `internal/runtime/zone/zone.go`（line 55 / 60 / 77）
- `internal/gateway/handler.go`（line 71 / 81 / 90）

**做完了是什么样**：
- zone.Spawn 内部：`LoadNPCTemplate(entry.TemplateRef)` → `ParseADMINTemplate` → `NewInstanceFromADMIN(id, pos, admin, src, btReg, compReg)`，compReg 从 Spawn 参数链路注入
- gateway handler 处理 spawn 请求同路径切换
- zone.go / handler.go 不再 import `npc.TemplateConfig` / `npc.ParseNPCTemplate` / `npc.NewInstanceFromTemplate` / `npc.LoadNPCTypeConfig`
- `go build ./...` 通过（未删 v2 源码前，zone.go/handler.go 与 template.go/compat.go 共存但不引用）

---

## T3: main.go 级联校验 + 双 spawn 路径收敛 (R15, R18)

**文件**：
- `cmd/server/main.go`

**做完了是什么样**：
- 新增 `validateCascadeDependencies(reg *npc.Registry) []string`，遍历每个 Instance 检查 `enable_emotion=true ∧ enable_memory=false` 级联违规，返回违规 NPC ID 列表
- 在 Registry 填充完成后、Scheduler 启动前调用；违规列表非空 → `log.Fatalf` 打印 NPC 列表 + ADMIN UI 修正路径，不跳过、不部分启动
- 移除 line 114-117 `if zm.Count() == 0` 互斥门：zone spawn（走 butterfly_01）与 ADMIN spawn（走 6 NPC）并行触发，两组 NPC 共存
- 启动日志 `zones.loaded count=1 npcs=3` + `admin.spawned count=6` 两行均出现

---

## T8: 删除 v2 生产路径源码 (R2, R3)

**文件**：
- `internal/runtime/npc/template.go`（整文件删除）
- `internal/runtime/npc/compat.go`（整文件删除）
- `internal/runtime/npc/compat_test.go`（整文件删除）

**做完了是什么样**：
- 3 文件物理删除
- `internal/runtime/npc/instance.go` 的 `NPCTypeConfig` / `ParseNPCTypeConfig` / `NewInstance` 保留（实验层 + 测试消费）
- `internal/config/source.go` 的 `LoadNPCTypeConfig` 接口方法保留（3 Source 实现保留）
- `go build ./...` + `go test ./...` 全绿
- `grep -r "NewInstanceFromTemplate\|ParseNPCTemplate\|TemplateConfig\|convertV2Format" internal/ cmd/` 返回零匹配

---

## T9: R14 孤儿字段透传集成测试

**文件**：
- `internal/runtime/admin_orphan_field_test.go`（新建）

**做完了是什么样**：
- 新增集成测试锚 snapshot §4 guard_basic 用例：构造 `ADMINTemplate{Fields: {"hp": 100}}` → `NewInstanceFromADMIN` → 断言 Instance.BB `hp=100` 通过 GetDynamic 可读
- 附带断言无 WARN 日志，hp 字段不阻塞 spawn
- 测试文件独立不混入其他集成测试文件

---

## T10: R15 / R16 验证 Gate

**文件**：无代码改动，仅手动执行与记录

**做完了是什么样**：
- **R15**：`docker compose up --build` 拉 ADMIN live（锚 `0aa77b2`），6 NPC（ADMIN）+ 3 butterfly（zone）共 9 NPC spawn 成功，tick ≥ 30s 无 WARN / ERROR 日志
- **R16**：断开 ADMIN（`docker compose stop admin` 或 `NPC_ADMIN_API=http://unreachable:1`），服务端降级到 JSONSource 从 `configs/` 加载，同样 9 NPC spawn（butterfly 走 zone，ADMIN 6 NPC 被 JSONSource 路径替代——此处行为需确认：若 JSONSource 无 ADMIN 6 NPC fixture，退化为仅 3 butterfly，R16 原文"同样 6 NPC"需放宽）
- smoke 结果记录到 PR 描述；若 FAIL 触发 `/debug`

---

## 验收总 Gate

以下全部通过后此 spec 关闭：

- [ ] `go test ./...` 全绿（含新增 T9 孤儿字段测试）
- [ ] `go build ./...` 通过
- [ ] `go run ./cmd/sync -api http://localhost:9821 && go test ./...` 仍全绿（R11 + R12）
- [ ] `docker compose up --build` 9 NPC spawn + tick ≥ 30s 无 WARN（R15）
- [ ] ADMIN 不可达场景 JSONSource 降级验证（R16，按 T10 修订条款）
- [ ] `grep` v2 符号零匹配（T8 验证）
- [ ] red-lines.md npctest 封闭红线已添加（T2）
