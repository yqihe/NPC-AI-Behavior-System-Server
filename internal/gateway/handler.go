package gateway

import (
	"encoding/json"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// RegisterHandlers 注册所有业务消息处理器
func RegisterHandlers(
	router *Router,
	registry *npc.Registry,
	bus *event.Bus,
	src config.Source,
	btReg *bt.Registry,
	evtTypes map[string]*event.EventTypeConfig,
) {
	router.Register(protocol.TypeSpawnNPC, makeSpawnNPCHandler(registry, src, btReg))
	router.Register(protocol.TypeRemoveNPC, makeRemoveNPCHandler(registry))
	router.Register(protocol.TypePublishEvent, makePublishEventHandler(bus, evtTypes))
	router.Register(protocol.TypeQueryNPC, makeQueryNPCHandler(registry))
}

func makeSpawnNPCHandler(registry *npc.Registry, src config.Source, btReg *bt.Registry) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.SpawnNPCRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse spawn_npc request")
			conn.sendMsg(resp)
			return nil
		}

		// 检查重复 ID
		if _, ok := registry.Get(req.NpcID); ok {
			resp, _ := protocol.NewError(msg.ID, "npc_already_exists", "NPC with id "+req.NpcID+" already exists")
			conn.sendMsg(resp)
			return nil
		}

		// 加载 NPC 类型配置
		rawCfg, err := src.LoadNPCTypeConfig(req.TypeName)
		if err != nil {
			slog.Warn("handler.spawn_npc.load_config", "type", req.TypeName, "err", err)
			resp, _ := protocol.NewError(msg.ID, "config_error", "failed to load NPC type: "+req.TypeName)
			conn.sendMsg(resp)
			return nil
		}
		typeCfg, err := npc.ParseNPCTypeConfig(rawCfg)
		if err != nil {
			slog.Warn("handler.spawn_npc.parse_config", "type", req.TypeName, "err", err)
			resp, _ := protocol.NewError(msg.ID, "config_error", "failed to parse NPC type config")
			conn.sendMsg(resp)
			return nil
		}

		// 创建 NPC 实例
		pos := event.Vec3{X: req.X, Z: req.Z}
		inst, err := npc.NewInstance(req.NpcID, pos, typeCfg, src, btReg)
		if err != nil {
			slog.Warn("handler.spawn_npc.create", "npc_id", req.NpcID, "err", err)
			resp, _ := protocol.NewError(msg.ID, "create_error", err.Error())
			conn.sendMsg(resp)
			return nil
		}

		registry.Add(inst)
		slog.Debug("handler.spawn_npc", "npc_id", req.NpcID, "type", req.TypeName)

		resp, _ := protocol.NewResponse(msg.ID, protocol.SpawnNPCResponse{
			NpcID:    req.NpcID,
			TypeName: req.TypeName,
		})
		conn.sendMsg(resp)
		return nil
	}
}

func makeRemoveNPCHandler(registry *npc.Registry) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.RemoveNPCRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse remove_npc request")
			conn.sendMsg(resp)
			return nil
		}

		if _, ok := registry.Get(req.NpcID); !ok {
			resp, _ := protocol.NewError(msg.ID, "npc_not_found", "NPC with id "+req.NpcID+" not found")
			conn.sendMsg(resp)
			return nil
		}

		registry.Remove(req.NpcID)
		slog.Debug("handler.remove_npc", "npc_id", req.NpcID)

		resp, _ := protocol.NewResponse(msg.ID, protocol.RemoveNPCResponse{
			NpcID: req.NpcID,
		})
		conn.sendMsg(resp)
		return nil
	}
}

func makePublishEventHandler(bus *event.Bus, evtTypes map[string]*event.EventTypeConfig) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.PublishEventRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse publish_event request")
			conn.sendMsg(resp)
			return nil
		}

		typeCfg, ok := evtTypes[req.EventType]
		if !ok {
			resp, _ := protocol.NewError(msg.ID, "unknown_event_type", "event type "+req.EventType+" not found")
			conn.sendMsg(resp)
			return nil
		}

		pos := event.Vec3{X: req.X, Z: req.Z}
		evt := event.NewEvent(typeCfg, pos, req.SourceID, req.Severity)
		bus.Publish(evt)
		slog.Debug("handler.publish_event", "event_id", evt.ID, "type", req.EventType)

		resp, _ := protocol.NewResponse(msg.ID, protocol.PublishEventResponse{
			EventID: evt.ID,
		})
		conn.sendMsg(resp)
		return nil
	}
}

func makeQueryNPCHandler(registry *npc.Registry) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.QueryNPCRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse query_npc request")
			conn.sendMsg(resp)
			return nil
		}

		inst, ok := registry.Get(req.NpcID)
		if !ok {
			resp, _ := protocol.NewError(msg.ID, "npc_not_found", "NPC with id "+req.NpcID+" not found")
			conn.sendMsg(resp)
			return nil
		}

		currentAction, _ := blackboard.Get(inst.BB, blackboard.KeyCurrentAction)
		threatLevel, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)

		resp, _ := protocol.NewResponse(msg.ID, protocol.QueryNPCResponse{
			NpcID:         inst.ID,
			TypeName:      inst.TypeName,
			X:             inst.Position.X,
			Z:             inst.Position.Z,
			FSMState:      inst.FSM.Current(),
			CurrentAction: currentAction,
			ThreatLevel:   threatLevel,
		})
		conn.sendMsg(resp)
		return nil
	}
}
