# 预期日志模式表 — e2e Full Integration 2026-04-21

供 Admin 对账脚本直接引用的正则表。日志全部走 `log/slog` 文本格式（`NPC_LOG_FORMAT=text`，compose 默认），形如：

```
time=2026-04-21T12:34:56.789+08:00 level=INFO msg=events.loaded count=5
```

**抓取命令**：`docker compose logs server --no-color`（多行 log 按 `\n` 分割后逐行 grep）。

**数据矩阵前提**（Admin 侧 seed，双边对齐）：

| 层 | 条数 | 说明 |
|----|------|------|
| event_types | 5 | earthquake / explosion / fire / gunshot / shout |
| FSM | 3 | fsm_combat_basic + fsm_passive + guard |
| BT | 6 | bt/combat/{idle,patrol,chase,attack} + bt/passive/wander + bt/guard/patrol |
| NPC 模板 | 5（其中 1 个 enabled=false） | e2e_bare / e2e_social / e2e_memo_emo / e2e_full / e2e_disabled |
| region | 2 | e2e_village（引 e2e_bare × 2）+ e2e_empty（空 spawn_table） |

**对账派生值**：
- `/api/configs/npc_templates` 端点返回 items.count = 4（disable fan-out 过滤）
- ADMIN 模板路径 spawn = 4
- Zone 路径 spawn = 2（e2e_village × 2 + e2e_empty × 0）
- **npc_active_count 总和 = 6**，其中 e2e_bare 有 3 个实例（1 模板路径 + 2 zone 路径）

**故障注入共享 fixture**（Admin `scripts/e2e/verify.sh` 硬编码，Server 正则不写死）：

| 场景 | 字段 | Admin 注入值 |
|------|------|-------------|
| R2A dangling region | `e2e_village.spawn_table[0].template_ref` | `missing_npc_xxx` |
| R2B dangling fsm_ref | `e2e_full.behavior.fsm_ref` | `missing_fsm_xxx` |

Server 正则使用 `\S+` 泛化捕获，Admin verify.sh 对 ref_value 做精确断言。如 Admin 变更 fixture，更新此表即可，Server 正则无需改动。

---

## 第一轮：happy path + disable fan-out

### 必须出现

| # | 锚点 | 正则 | 期望 | 判定 |
|---|------|------|------|------|
| 1.1 | 配置源标注 | `msg=config\.source type=http base_url=http://[^ ]+` | 1 行 | 精确 1 |
| 1.2 | event_types 加载 | `msg=config\.http\.loaded endpoint=/api/configs/event_types count=(\d+)` | count=5 | 捕获组 == 5 |
| 1.3 | fsm_configs 加载 | `msg=config\.http\.loaded endpoint=/api/configs/fsm_configs count=(\d+)` | count=3 | 捕获组 == 3 |
| 1.4 | bt_trees 加载 | `msg=config\.http\.loaded endpoint=/api/configs/bt_trees count=(\d+)` | count=6 | 捕获组 == 6 |
| 1.5 | npc_templates 加载 | `msg=config\.http\.loaded endpoint=/api/configs/npc_templates count=(\d+)` | count=4 | 捕获组 == 4 |
| 1.6 | regions 加载 | `msg=config\.http\.loaded endpoint=/api/configs/regions count=(\d+)` | count=2 | 捕获组 == 2 |
| 1.7 | events 注册数 | `msg=events\.loaded count=(\d+)` | count=5 | 捕获组 == 5 |
| 1.8 | zones 加载数 | `msg=zones\.loaded count=(\d+)` | count=2 | 捕获组 == 2 |
| 1.9 | ADMIN 模板 spawn 结果 | `msg=admin_spawn\.done spawned=(\d+) template_count=(\d+)` | spawned=4 template_count=4 | 两捕获组均 == 4 |
| 1.10 | 服务启动监听 | `msg=server\.start addr=:9820` | 1 行 | 精确 1 |

### 不得出现

| # | 锚点 | 正则 | 期望 |
|---|------|------|------|
| 1.11 | 配置源错误 | `msg=config\.http_error` | 0 行 |
| 1.12 | regions dangling | `msg=config\.http\.regions\.dangling` | 0 行 |
| 1.13 | zone spawn 错误 | `msg=zones\.spawn_error` | 0 行 |
| 1.14 | 模板解析错误 | `msg=admin_spawn\.parse_error` | 0 行 |
| 1.15 | 模板实例化错误 | `msg=admin_spawn\.instance_error` | 0 行 |
| 1.16 | 级联违规 | `msg=cascade\.violations` | 0 行 |

### `/metrics` 对账（tick 稳定后）

**等待**：`sleep 1` 确保 ≥ 10 个 tick（tick_rate=100ms）已跑。

| # | 源 | 正则 | 期望 | 判定 |
|---|-----|------|------|------|
| 1.17 | curl `http://localhost:9820/metrics` | `^npc_active_count(?:\{zone="[^"]*"\})?\s+(\d+)$` | 所有匹配行捕获组求和 = 6 | 总和精确等 |
| 1.18 | 同上 | `^npc_tick_total\s+(\d+)$` | 捕获组 ≥ 10 | 非零且增长 |

---

## 第二轮之一：dangling region

**操作**：Admin 把 `e2e_village.spawn_table[0].template_ref` 改为 `missing_npc_xxx`（见 §故障注入共享 fixture），`docker compose restart server`。

### 必须出现

| # | 锚点 | 正则 | 期望 | 判定 |
|---|------|------|------|------|
| 2A.1 | 前 4 端点 loaded | 同 1.2–1.5（四行） | count 各项正常 | 逐行捕获组相等 |
| 2A.2 | 悬空引用详情 | `msg=config\.http\.regions\.dangling region_id=e2e_village ref_type=\S+ ref_value=\S+ reason=\S+` | ≥ 1 行 | 至少 1（ref_value 精确值见 fixture 表） |
| 2A.3 | 汇总错误行 | `msg=config\.http_error err=".*code=47011.*"` | 1 行 | 精确 1 |
| 2A.4 | 容器重启循环 | `docker inspect --format='{{.RestartCount}}' <server-container>` | ≥ 2 | 数值比较（非日志） |

### 不得出现

| # | 锚点 | 正则 | 期望 |
|---|------|------|------|
| 2A.5 | regions loaded 行 | `msg=config\.http\.loaded endpoint=/api/configs/regions` | 0 行 |
| 2A.6 | zones.loaded | `msg=zones\.loaded` | 0 行 |
| 2A.7 | admin_spawn.done | `msg=admin_spawn\.done` | 0 行 |
| 2A.8 | server.start | `msg=server\.start` | 0 行 |

---

## 第二轮之二：dangling fsm_ref

**操作**：Admin 恢复 region，把 `e2e_full.behavior.fsm_ref` 改为 `missing_fsm_xxx`，`docker compose restart server`。

### 必须出现

| # | 锚点 | 正则 | 期望 | 判定 |
|---|------|------|------|------|
| 2B.1 | 前 3 端点 loaded | 同 1.2–1.4（三行） | count 各项正常 | 逐行捕获组相等 |
| 2B.2 | 悬空引用详情 | `msg=config\.http\.npc_templates\.dangling npc_name=\S+ ref_type=\S+ ref_value=\S+ reason=\S+` | ≥ 1 行 | 至少 1（R2B fixture 产出 `npc_name=e2e_full ref_type=fsm_ref ref_value=missing_fsm_xxx`） |
| 2B.3 | 汇总错误行 | `msg=config\.http_error err=".*code=45016.*"` | 1 行 | 精确 1 |
| 2B.4 | 容器重启循环 | `docker inspect --format='{{.RestartCount}}' <server-container>` | ≥ 2 | 数值比较（非日志） |

### 不得出现

| # | 锚点 | 正则 | 期望 |
|---|------|------|------|
| 2B.5 | npc_templates loaded 行 | `msg=config\.http\.loaded endpoint=/api/configs/npc_templates` | 0 行 |
| 2B.6 | regions loaded 行 | `msg=config\.http\.loaded endpoint=/api/configs/regions` | 0 行 |
| 2B.7 | zones.loaded | `msg=zones\.loaded` | 0 行 |
| 2B.8 | admin_spawn.done | `msg=admin_spawn\.done` | 0 行 |
| 2B.9 | server.start | `msg=server\.start` | 0 行 |

> **注**：`ref_type=bt_ref` 时 details 会多带一个 `state=<str>` 字段（Admin 契约：仅 BT 状态绑定悬空时填充）。2B.2 正则不断言 state 存在，以兼容 fsm_ref（无 state）与 bt_ref（有 state）两种产出。

---

## 日志级别建议

`.env.e2e` 建议 `NPC_LOG_LEVEL=info`（而非 compose 默认的 `debug`）。原因：

- `debug` 级别会打 tick / broadcast / hub.register 等高频行，对 happy path 对账无帮助且放大日志体积
- 故障注入场景启动早期就 `os.Exit`，级别影响不大

如果对账时需要确认 websocket 建连或 tick 内部状态，临时把级别切回 `debug` 单独跑一轮，但不作对账主路径。
