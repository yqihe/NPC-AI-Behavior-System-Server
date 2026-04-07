package gateway

import (
	"encoding/json"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// RegisterHandlers 注册所有业务消息处理器
func RegisterHandlers(
	router *Router,
	registry *npc.Registry,
	bus *event.Bus,
	src config.Source,
	btReg *bt.Registry,
	compReg *component.Registry,
	gm *social.GroupManager,
	evtTypes map[string]*event.EventTypeConfig,
) {
	router.Register(protocol.TypeSpawnNPC, makeSpawnNPCHandler(registry, src, btReg, compReg, gm))
	router.Register(protocol.TypeRemoveNPC, makeRemoveNPCHandler(registry, gm))
	router.Register(protocol.TypePublishEvent, makePublishEventHandler(bus, evtTypes))
	router.Register(protocol.TypeQueryNPC, makeQueryNPCHandler(registry))
}

func makeSpawnNPCHandler(registry *npc.Registry, src config.Source, btReg *bt.Registry, compReg *component.Registry, gm *social.GroupManager) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.SpawnNPCRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse spawn_npc request")
			conn.sendMsg(resp)
			return nil
		}

		// 校验必填字段
		if req.NpcID == "" {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "npc_id cannot be empty")
			conn.sendMsg(resp)
			return nil
		}
		if req.TypeName == "" {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "type_name cannot be empty")
			conn.sendMsg(resp)
			return nil
		}

		// 检查重复 ID
		if _, ok := registry.Get(req.NpcID); ok {
			resp, _ := protocol.NewError(msg.ID, "npc_already_exists", "NPC with id "+req.NpcID+" already exists")
			conn.sendMsg(resp)
			return nil
		}

		pos := event.Vec3{X: req.X, Z: req.Z}

		// 尝试加载配置：先新格式（npc_templates），再旧格式（npc_types）
		rawCfg, err := src.LoadNPCTemplate(req.TypeName)
		if err != nil {
			// 降级到旧格式
			rawCfg, err = src.LoadNPCTypeConfig(req.TypeName)
			if err != nil {
				slog.Warn("handler.spawn_npc.load_config", "type", req.TypeName, "err", err)
				resp, _ := protocol.NewError(msg.ID, "config_error", "failed to load NPC type: "+req.TypeName)
				conn.sendMsg(resp)
				return nil
			}
		}

		// 统一解析（自动检测新旧格式）
		tmpl, err := npc.ParseNPCTemplate(rawCfg)
		if err != nil {
			slog.Warn("handler.spawn_npc.parse_config", "type", req.TypeName, "err", err)
			resp, _ := protocol.NewError(msg.ID, "config_error", "failed to parse NPC config")
			conn.sendMsg(resp)
			return nil
		}

		// 创建组件化 NPC 实例
		inst, err := npc.NewInstanceFromTemplate(req.NpcID, pos, tmpl, compReg, src, btReg)
		if err != nil {
			slog.Warn("handler.spawn_npc.create", "npc_id", req.NpcID, "err", err)
			resp, _ := protocol.NewError(msg.ID, "create_error", "failed to create NPC instance")
			conn.sendMsg(resp)
			return nil
		}

		registry.Add(inst)
		if gm != nil {
			gm.Register(inst)
		}
		slog.Debug("handler.spawn_npc", "npc_id", req.NpcID, "type", req.TypeName)

		resp, _ := protocol.NewResponse(msg.ID, protocol.SpawnNPCResponse{
			NpcID:    req.NpcID,
			TypeName: req.TypeName,
		})
		conn.sendMsg(resp)
		return nil
	}
}

func makeRemoveNPCHandler(registry *npc.Registry, gm *social.GroupManager) HandlerFunc {
	return func(conn *Conn, msg *protocol.Message) error {
		var req protocol.RemoveNPCRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "failed to parse remove_npc request")
			conn.sendMsg(resp)
			return nil
		}

		if req.NpcID == "" {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "npc_id cannot be empty")
			conn.sendMsg(resp)
			return nil
		}

		inst, ok := registry.Get(req.NpcID)
		if !ok {
			resp, _ := protocol.NewError(msg.ID, "npc_not_found", "NPC with id "+req.NpcID+" not found")
			conn.sendMsg(resp)
			return nil
		}

		if gm != nil {
			gm.Unregister(inst)
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

		if req.EventType == "" {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "event_type cannot be empty")
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
		evt := event.NewEvent(typeCfg, pos, req.SourceID, req.Severity, req.ZoneID)
		bus.Publish(evt)
		slog.Debug("handler.publish_event", "event_id", evt.ID, "type", req.EventType, "zone_id", req.ZoneID)

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

		if req.NpcID == "" {
			resp, _ := protocol.NewError(msg.ID, "invalid_data", "npc_id cannot be empty")
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

		// 从 behavior 组件安全读取 FSM 状态
		fsmState := ""
		if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok && beh.FSM != nil {
			fsmState = beh.FSM.Current()
		} else if inst.FSM != nil {
			fsmState = inst.FSM.Current()
		}

		resp, _ := protocol.NewResponse(msg.ID, protocol.QueryNPCResponse{
			NpcID:         inst.ID,
			TypeName:      inst.TypeName,
			X:             inst.Position.X,
			Z:             inst.Position.Z,
			FSMState:      fsmState,
			CurrentAction: currentAction,
			ThreatLevel:   threatLevel,
		})
		conn.sendMsg(resp)
		return nil
	}
}
