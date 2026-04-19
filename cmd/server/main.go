package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/gateway"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/metrics"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/zone"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// serverConfig 服务端启动配置
type serverConfig struct {
	Addr             string  `json:"addr"`
	TickRateMs       int     `json:"tick_rate_ms"`
	DecisionDecayRate float64 `json:"decision_decay_rate"`
	LogLevel         string  `json:"log_level"`
	LogFormat        string  `json:"log_format"`
	MongoURI         string  `json:"mongo_uri"`
	AdminAPI         string  `json:"admin_api"`
}

func main() {
	// 1. 加载服务端配置
	cfg := loadServerConfig("configs/server.json")
	applyEnvOverrides(&cfg)
	initLogger(cfg.LogLevel, cfg.LogFormat)

	slog.Info("config.loaded",
		"addr", cfg.Addr,
		"tick_rate_ms", cfg.TickRateMs,
		"decay_rate", cfg.DecisionDecayRate,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
	)

	// 2. 初始化配置源（优先级：AdminAPI > MongoURI > JSONSource）
	var src config.Source
	if cfg.AdminAPI != "" {
		httpSrc, err := config.NewHTTPSource(context.Background(), cfg.AdminAPI)
		if err != nil {
			slog.Error("config.http_error", "err", err)
			os.Exit(1)
		}
		src = httpSrc
		slog.Info("config.source", "type", "http", "base_url", cfg.AdminAPI)
	} else if cfg.MongoURI != "" {
		mongoSrc, err := config.NewMongoSource(context.Background(), cfg.MongoURI, "npc_ai")
		if err != nil {
			slog.Error("config.mongo_error", "err", err)
			os.Exit(1)
		}
		src = mongoSrc
		slog.Info("config.source", "type", "mongodb")
	} else {
		src = config.NewJSONSource("configs")
		slog.Info("config.source", "type", "json", "dir", "configs")
	}

	// 3. 加载事件类型配置
	evtTypes := loadAllEventTypes(src)
	slog.Info("events.loaded", "count", len(evtTypes))

	// 4. 初始化 Runtime
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	bus := event.NewBus()
	reg := npc.NewRegistry()
	dec := decision.NewCenter(cfg.DecisionDecayRate)
	gm := social.NewGroupManager()
	zm := zone.NewZoneManager()
	tickRate := time.Duration(cfg.TickRateMs) * time.Millisecond
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, tickRate)
	m := metrics.New()
	sched.GroupManager = gm
	sched.ZoneManager = zm
	sched.Metrics = m

	// 4a. 加载区域配置并批量生成 NPC（组件化路径）
	regionConfigs, err := src.LoadAllRegionConfigs()
	if err != nil {
		slog.Warn("zones.load_error", "err", err)
	}
	for _, data := range regionConfigs {
		var z zone.Zone
		if err := json.Unmarshal(data, &z); err != nil {
			slog.Error("zones.parse_error", "err", err)
			continue
		}
		z.Active = true
		zm.AddZone(&z)
		if err := z.Spawn(compReg, src, btReg, reg, gm); err != nil {
			slog.Error("zones.spawn_error", "zone", z.ID, "err", err)
		}
	}
	slog.Info("zones.loaded", "count", zm.Count())

	// 4b. ADMIN 模板 spawn（与 zone spawn 并行触发；design §1.3 双路径收敛 R15）
	spawnFromADMINTemplates(src, btReg, compReg, reg)

	// 4c. R18 级联依赖校验：enable_emotion=true ∧ enable_memory=false 违规 → fatal
	if violations := validateCascadeDependencies(reg); len(violations) > 0 {
		slog.Error("cascade.violations",
			"count", len(violations),
			"npc_ids", violations,
			"fix_path", "ADMIN UI → 模板 → 能力开关 → enable_memory 打开（emotion 依赖 memory）",
		)
		os.Exit(1)
	}

	// 5. 初始化 Gateway
	hub := gateway.NewHub()
	router := gateway.NewRouter()
	gateway.RegisterHandlers(router, reg, bus, src, btReg, compReg, gm, evtTypes)
	srv := gateway.NewServer(cfg.Addr, hub, router)
	srv.MetricsHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(m.PrometheusText()))
	}

	// 6. 启动
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go sched.Run(ctx)
	go hub.Run(ctx)
	go broadcastLoop(ctx, hub, reg, tickRate)

	if err := srv.Start(ctx); err != nil {
		slog.Error("server.fatal", "err", err)
		os.Exit(1)
	}
}

// loadServerConfig 从 JSON 文件加载服务端配置
func loadServerConfig(path string) serverConfig {
	cfg := serverConfig{
		Addr:             ":9820",
		TickRateMs:       100,
		DecisionDecayRate: 5.0,
		LogLevel:         "debug",
		LogFormat:        "text",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("config.file_not_found", "path", path, "err", err)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("config.parse_error", "path", path, "err", err)
	}
	return cfg
}

// applyEnvOverrides 环境变量覆盖配置
func applyEnvOverrides(cfg *serverConfig) {
	if v := os.Getenv("NPC_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("NPC_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("NPC_LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("NPC_MONGO_URI"); v != "" {
		cfg.MongoURI = v
	}
	if v := os.Getenv("NPC_ADMIN_API"); v != "" {
		cfg.AdminAPI = v
	}
}

// initLogger 初始化 slog
func initLogger(level, format string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// validateCascadeDependencies 遍历 Registry 逐 NPC 检查 R18 组件级联违规。
// 当前仅一条硬依赖：emotion 需要 memory（emotion.Tick 读 KeyMemoryThreatValue
// 由 memory.Tick 写入）。违规 NPC ID 列表返回给调用方；非空则 main 退出。
// 校验基于 Instance.HasComponent — 组件存在等价于 R17 opt-in 加载时
// enable_*=true，与 ADMIN fields.enable_* 语义等价。
func validateCascadeDependencies(reg *npc.Registry) []string {
	var violations []string
	reg.ForEach(func(inst *npc.Instance) {
		if inst.HasComponent("emotion") && !inst.HasComponent("memory") {
			violations = append(violations, inst.ID)
		}
	})
	return violations
}

// spawnFromADMINTemplates 从 ADMIN 形状模板批量 spawn NPC。
// 每个模板创建 1 个实例，按索引排布在 10m 网格上（y=0 平面）。
// 无模板时无副作用。模板解析/创建失败逐条告警不中断。
func spawnFromADMINTemplates(src config.Source, btReg *bt.Registry, compReg *component.Registry, reg *npc.Registry) {
	tmpls, err := src.LoadAllNPCTemplates()
	if err != nil {
		slog.Warn("admin_spawn.load_error", "err", err)
		return
	}
	if len(tmpls) == 0 {
		slog.Info("admin_spawn.skipped", "reason", "no templates")
		return
	}

	idx := 0
	for name, data := range tmpls {
		tmpl, err := npc.ParseADMINTemplate(name, data)
		if err != nil {
			slog.Warn("admin_spawn.parse_error", "template", name, "err", err)
			continue
		}
		pos := event.Vec3{X: float64(idx%10) * 10, Z: float64(idx/10) * 10}
		inst, err := npc.NewInstanceFromADMIN(fmt.Sprintf("%s_%d", name, idx), pos, tmpl, src, btReg, compReg)
		if err != nil {
			slog.Warn("admin_spawn.instance_error", "template", name, "err", err)
			continue
		}
		reg.Add(inst)
		idx++
	}
	slog.Info("admin_spawn.done", "spawned", idx, "template_count", len(tmpls))
}

// loadAllEventTypes 加载所有事件类型配置
func loadAllEventTypes(src config.Source) map[string]*event.EventTypeConfig {
	rawConfigs, err := src.LoadAllEventConfigs()
	if err != nil {
		slog.Error("events.load_error", "err", err)
		os.Exit(1)
	}

	evtTypes := make(map[string]*event.EventTypeConfig, len(rawConfigs))
	for name, data := range rawConfigs {
		var cfg event.EventTypeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			slog.Error("events.parse_error", "name", name, "err", err)
			os.Exit(1)
		}
		evtTypes[cfg.Name] = &cfg
		slog.Debug("events.loaded_type", "name", cfg.Name, "severity", cfg.DefaultSeverity)
	}
	return evtTypes
}

// broadcastLoop 按 tickRate 频率广播 WorldSnapshot
func broadcastLoop(ctx context.Context, hub *gateway.Hub, reg *npc.Registry, tickRate time.Duration) {
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	var tickCount uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCount++
			if hub.Count() == 0 {
				continue
			}

			snapshot := buildSnapshot(tickCount, reg)
			snapshotJSON, err := json.Marshal(snapshot)
			if err != nil {
				slog.Warn("broadcast.marshal_error", "err", err)
				continue
			}

			msg, err := json.Marshal(protocol.Message{
				Type: protocol.TypeWorldSnapshot,
				Data: json.RawMessage(snapshotJSON),
			})
			if err != nil {
				slog.Warn("broadcast.marshal_error", "err", err)
				continue
			}

			hub.Broadcast(msg)
			slog.Debug("broadcast.snapshot", "tick", tickCount, "npc_count", len(snapshot.NPCs))
		}
	}
}

// buildSnapshot 构建当前世界状态快照
func buildSnapshot(tick uint64, reg *npc.Registry) protocol.WorldSnapshot {
	npcs := make([]protocol.NPCState, 0) // 保证 JSON 序列化为 [] 而非 null

	reg.ForEach(func(inst *npc.Instance) {
		currentAction, _ := blackboard.Get(inst.BB, blackboard.KeyCurrentAction)
		threatLevel, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)

		// 从 behavior 组件安全读取 FSM 状态
		fsmState := ""
		if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok && beh.FSM != nil {
			fsmState = beh.FSM.Current()
		} else if inst.FSM != nil {
			fsmState = inst.FSM.Current()
		}

		npcs = append(npcs, protocol.NPCState{
			NpcID:         inst.ID,
			TypeName:      inst.TypeName,
			X:             inst.Position.X,
			Z:             inst.Position.Z,
			FSMState:      fsmState,
			CurrentAction: currentAction,
			ThreatLevel:   threatLevel,
		})
	})

	return protocol.WorldSnapshot{
		Tick: tick,
		NPCs: npcs,
	}
}
