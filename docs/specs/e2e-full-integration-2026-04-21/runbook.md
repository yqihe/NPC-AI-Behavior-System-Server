# Server-v1 e2e Full Integration Runbook — 2026-04-21

本文档描述 Server-v1 在 **全链路从零构建认知 e2e 验收**（与 Admin 联调）中的职责：重置手顺、启动顺序、观测锚点清单、故障注入步骤。配套的 Admin 侧脚本、seed 数据、对账脚本在 Admin 仓 `docs/specs/e2e-full-integration-2026-04-21/` 下。

## 验收目标（L2 加载级）

1. Server 能从 ADMIN HTTP 源拉取 5 端点全部配置
2. NPC 按双路径 spawn 收敛（ADMIN 模板全量 + zone spawn_table）
3. 5 个 opt-in 组件按字段开关装配（bare / social / memory+emotion / full stack）
4. 故障注入两条：`dangling region.template_ref` 与 `dangling npc_template.fsm_ref` 均 fail-fast 且容器重启循环

非目标：L3 运行级（FSM 状态转换、BT 节点轨迹、perception 事件分发）本轮不验。

## 配置源假设

- Server 走 **HTTPSource** 模式（`NPC_ADMIN_API=http://npc-admin-backend:9821`）
- 走 HTTPSource 时 Server **完全不读** `configs/` 目录 — 本地 fixture 与联调解耦
- 日志格式 `NPC_LOG_FORMAT=text`（compose 默认），级别建议 `NPC_LOG_LEVEL=info`（减少 tick/broadcast 噪声）

## Reset 手顺

Server 侧无持久化（runtime 全在内存），reset 等价于重启容器：

```bash
docker compose --env-file .env.e2e down
docker compose --env-file .env.e2e up --build -d
```

`.env.e2e`（不入库，`.env.*` 被 .gitignore 兜住）：

```env
NPC_PORT=9820
NPC_LOG_LEVEL=info
NPC_LOG_FORMAT=text
NPC_MONGO_URI=
NPC_ADMIN_API=http://npc-admin-backend:9821
MONGO_PORT=27017
```

> **注**：若 Admin 后端跑在宿主机而非容器，改 `NPC_ADMIN_API=http://host.docker.internal:9821`。

## 启动顺序

1. Admin 侧先完成 seed（5 event_types / 3 FSM / 6 BT / 5 NPC 模板其中 1 个 `enabled=false` / 2 region），端点 curl 自检通过
2. Server 侧 `docker compose --env-file .env.e2e up --build -d`
3. **等待 ≥ 1s** 让 scheduler 跑至少 10 个 tick，`metrics.RecordTick` 稳定
4. Admin 侧对账脚本：curl `/metrics` + `docker compose logs server` 抓日志 → 正则匹配 → PASS/FAIL

## 观测锚点清单

日志全部经 `log/slog` 结构化输出到 stdout，格式 `time=... level=XXX msg=<组件.动作> key1=val1 key2=val2 ...`。

### 启动阶段顺序（happy path）

按出现顺序列：

| # | msg | 关键 kv | 作用 |
|---|-----|--------|------|
| 1 | `config.loaded` | `addr=:9820 tick_rate_ms=100 ...` | 服务端配置文件已加载 |
| 2 | `config.http.loaded` × 5 | `endpoint=/api/configs/<name> count=<n>` | 5 端点拉取成功，每端点一行（event_types → fsm_configs → bt_trees → npc_templates → regions） |
| 3 | `config.source` | `type=http base_url=<admin_url>` | **O4 生效源标注**，HTTPSource 构造完成后打出（**位置在 5 端点之后**） |
| 4 | `events.loaded` | `count=<n>` | 事件类型最终注册数 |
| 5 | `zone.spawned` × N | `zone=<id> npc_count=<n>` | 每个 region 的 zone spawn 详情 |
| 6 | `zones.loaded` | `count=<n>` | 区域加载完成（含空 spawn_table） |
| 7 | `admin_spawn.done` | `spawned=<n> template_count=<n>` | ADMIN 模板路径 spawn 完成 |
| 8 | `server.start` | `addr=:9820` | HTTP/WS 监听成功 |

> 对账用正则存在性 + 捕获组判定，不按行序卡 PASS/FAIL —— 上表供排障时快速定位启动卡点。

### 双路径 spawn 收敛（design §1.3 R15）

Server 启动会**同时触发两条 spawn 路径**，对账必须叠加：

| 路径 | 数据来源 | spawn 数 |
|------|---------|---------|
| Zone spawn（`cmd/server/main.go:105-116`） | `/api/configs/regions` 每个 region 的 `spawn_table[]` | Σ region.spawn_table.count |
| ADMIN 模板 spawn（`main.go:120` → `spawnFromADMINTemplates`） | `/api/configs/npc_templates` 全量模板**逐个**实例化一份 | len(templates) |

**npc_active_count 对账公式** = Σ region.spawn_table.count + len(npc_templates)

### 失败锚点（必须全部 0 行）

| msg | 含义 |
|-----|------|
| `config.http_error` | HTTPSource 拉取失败（任一 mandatory 端点） → `os.Exit(1)` |
| `config.http.regions.dangling` | regions 端点 500 + 业务码 47011 的 details 解码（见 PR #37） |
| `config.http.npc_templates.dangling` | npc_templates 端点 500 + 业务码 45016 的 details 解码（悬空 fsm_ref / bt_ref） |
| `zones.spawn_error` | 某 region spawn 过程报错 |
| `admin_spawn.parse_error` | 模板 JSON 解析失败 |
| `admin_spawn.instance_error` | 模板实例化失败（BT/组件/FSM 绑定错） |
| `cascade.violations` | R18 级联依赖违规（enable_emotion=true ∧ enable_memory=false） → `os.Exit(1)` |

## 故障注入

### 第二轮之一：dangling region

**操作**：Admin 侧把 `e2e_village.spawn_table[0].template_ref` 从 `e2e_bare` 改成 `missing_npc_xxx`（Admin `scripts/e2e/verify.sh` 共享 fixture，见 expected-log-patterns.md §故障注入共享 fixture）；Server 不变，仅 `docker compose restart server`。

**Server 端预期**：
- 前 4 端点（event_types/fsm_configs/bt_trees/npc_templates）正常 `config.http.loaded`
- regions 端点拉取时 HTTPSource `fetchRegionsEndpoint` 检测到 HTTP 500 + code=47011
- 按 `details[]` 逐条打：`config.http.regions.dangling region_id=e2e_village ref_type=<str> ref_value=missing_npc_xxx reason=<str>`
  > 注意：Admin 侧 `details[].npc_name` 字段在 regions 语境下实际承载 region_id（memory: Admin regions 端点契约）—— Server 端已适配，日志输出的 key 叫 `region_id`
- `config.http_error err="config: regions export dangling refs (code=47011, count=<n>): <msg>"`
- `main.go:64` `os.Exit(1)` → 容器重启循环（`docker inspect --format='{{.RestartCount}}' <container>` ≥ 2）
- **不会出现**：`zones.loaded`、`admin_spawn.done`、`server.start`（启动未走到后续阶段）

### 第二轮之二：dangling fsm_ref

**操作**：Admin 恢复 region，把 `e2e_full.behavior.fsm_ref` 改成 `missing_fsm_xxx`；Server 侧 restart。

**Server 端预期**：
- `event_types` / `fsm_configs` / `bt_trees` 三端点正常 loaded
- `npc_templates` 端点拉取：HTTP 500（Admin 侧返 code=45016）→ `fetchNpcTemplatesEndpoint` 检测到业务码 45016
- 按 `details[]` 逐条打：`config.http.npc_templates.dangling npc_name=<npc> ref_type=<fsm_ref|bt_ref> ref_value=<missing_name> reason=<str>`（`ref_type=bt_ref` 时额外带 `state=<str>`）
  > Admin 契约：details[] 字段名与 regions 端点共享 struct，但 `npc_name` 在此端点下**字面承载 NPC 名**（不像 regions 那样遗留），无需重命名
- `config.http_error err="config: npc_templates export dangling refs (code=45016, count=<n>): <msg>"`
- `main.go:64` `os.Exit(1)` → 容器重启循环（`docker inspect --format='{{.RestartCount}}' <container>` ≥ 2）
- **不会出现**：`config.http.loaded endpoint=/api/configs/npc_templates`、`config.http.loaded endpoint=/api/configs/regions`、`zones.loaded`、`admin_spawn.done`、`server.start`（启动未走到后续阶段）

> **其他 3 端点（event_types / fsm_configs / bt_trees）的 5xx**：Admin 契约确认为叶子导出，无跨模块 ref 校验，500 永远是通用 `ErrInternal=50000`，无 details 可解。Server 侧这 3 端点继续走通用 `fetchEndpoint`，日志打 `status 500 body=<raw>`，不引入 45016 路径（避免误匹配）。

## 产出物

本目录下：
- `runbook.md` — 本文件
- `expected-log-patterns.md` — 预期日志模式表（正则版，供 Admin 对账脚本直接引用）
- `joint-report.md` — 双边 e2e 跑完后填（结果与 Admin 仓独立 git 追踪，内容保持一致）

## 参考

- [CLAUDE.md](../../../CLAUDE.md) § 技术栈 / 架构约束
- [docs/standards/acceptance-checklist.md](../../standards/acceptance-checklist.md) § 五 配置与数据源 (CF1/CF2/CF3)
- [PR #37](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/37) regions 端点 47011 fail-fast
- Memory: `reference_admin_regions_api.md`（envelope 形状 + npc_name 承载 region_id 的坑）
