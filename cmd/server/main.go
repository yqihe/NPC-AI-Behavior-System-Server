package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/gateway"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// serverConfig 服务端启动配置
type serverConfig struct {
	Addr             string  `json:"addr"`
	TickRateMs       int     `json:"tick_rate_ms"`
	DecisionDecayRate float64 `json:"decision_decay_rate"`
	LogLevel         string  `json:"log_level"`
	LogFormat        string  `json:"log_format"`
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

	// 2. 初始化配置源（唯一来源：ADMIN API）
	if cfg.AdminAPI == "" {
		slog.Error("config.missing", "msg", "NPC_ADMIN_API is required")
		os.Exit(1)
	}
	src, err := config.NewHTTPSource(context.Background(), cfg.AdminAPI)
	if err != nil {
		slog.Error("config.http_error", "err", err)
		os.Exit(1)
	}
	slog.Info("config.source", "type", "http", "base_url", cfg.AdminAPI)

	// 3. 加载事件类型配置
	evtTypes := loadAllEventTypes(src)
	slog.Info("events.loaded", "count", len(evtTypes))

	// 4. 初始化 Runtime
	btReg := bt.DefaultRegistry()
	bus := event.NewBus()
	reg := npc.NewRegistry()
	dec := decision.NewCenter(cfg.DecisionDecayRate)
	tickRate := time.Duration(cfg.TickRateMs) * time.Millisecond
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, tickRate)

	// 5. 初始化 Gateway
	hub := gateway.NewHub()
	router := gateway.NewRouter()
	gateway.RegisterHandlers(router, reg, bus, src, btReg, evtTypes)
	srv := gateway.NewServer(cfg.Addr, hub, router)

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

		npcs = append(npcs, protocol.NPCState{
			NpcID:         inst.ID,
			TypeName:      inst.TypeName,
			X:             inst.Position.X,
			Z:             inst.Position.Z,
			FSMState:      inst.FSM.Current(),
			CurrentAction: currentAction,
			ThreatLevel:   threatLevel,
		})
	})

	return protocol.WorldSnapshot{
		Tick: tick,
		NPCs: npcs,
	}
}
