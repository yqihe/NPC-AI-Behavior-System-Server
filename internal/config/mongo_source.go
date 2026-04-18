package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
)

// 确保 MongoSource 实现 Source 接口
var _ Source = (*MongoSource)(nil)

// configDoc MongoDB 中配置文档的通用结构
type configDoc struct {
	Name   string   `bson:"name"`
	Config bson.Raw `bson:"config"`
}

// MongoSource 从 MongoDB 加载配置，启动时全量加载到内存
type MongoSource struct {
	npcTypes     map[string][]byte // name → raw JSON（v2 兼容）
	npcTemplates map[string][]byte // name → raw JSON（v3 组件化）
	fsmConfigs   map[string][]byte // name → raw JSON
	btTrees      map[string][]byte // name → raw JSON
	eventTypes   map[string][]byte // name → raw JSON
}

// NewMongoSource 连接 MongoDB，全量加载配置到内存，然后断开连接
func NewMongoSource(ctx context.Context, uri, database string) (*MongoSource, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("config: mongo connect: %w", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("config: mongo ping: %w", err)
	}

	db := client.Database(database)

	s := &MongoSource{
		npcTypes:     make(map[string][]byte),
		npcTemplates: make(map[string][]byte),
		fsmConfigs:   make(map[string][]byte),
		btTrees:      make(map[string][]byte),
		eventTypes:   make(map[string][]byte),
	}

	loaders := []struct {
		collection string
		target     map[string][]byte
		optional   bool
	}{
		{"event_types", s.eventTypes, false},
		{"npc_types", s.npcTypes, false},
		{"fsm_configs", s.fsmConfigs, false},
		{"bt_trees", s.btTrees, false},
		{"npc_templates", s.npcTemplates, true},
	}

	for _, l := range loaders {
		if err := loadCollection(ctx, db, l.collection, l.target); err != nil {
			if l.optional {
				slog.Warn("config.mongo.optional_collection_skipped", "collection", l.collection, "err", err)
				continue
			}
			return nil, err
		}
		slog.Info("config.mongo.loaded", "collection", l.collection, "count", len(l.target))
	}

	return s, nil
}

// loadCollection 从一个 collection 加载所有文档到 map
func loadCollection(ctx context.Context, db *mongo.Database, name string, target map[string][]byte) error {
	coll := db.Collection(name)
	cursor, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("config: mongo find %s: %w", name, err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc configDoc
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("config: mongo decode %s: %w", name, err)
		}
		if doc.Name == "" {
			continue
		}
		// bson.Raw → JSON bytes
		jsonBytes, err := bson.MarshalExtJSON(doc.Config, false, false)
		if err != nil {
			return fmt.Errorf("config: mongo marshal %s/%s: %w", name, doc.Name, err)
		}
		target[doc.Name] = jsonBytes
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("config: mongo cursor %s: %w", name, err)
	}

	if len(target) == 0 {
		return fmt.Errorf("config: mongo collection %s is empty", name)
	}

	return nil
}

// --- Source 接口实现（纯内存读取）---

func (s *MongoSource) LoadFSMConfig(npcType string) (*fsm.FSMConfig, error) {
	data, ok := s.fsmConfigs[npcType]
	if !ok {
		return nil, fmt.Errorf("config: FSM %q not found in MongoDB", npcType)
	}
	var cfg fsm.FSMConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse FSM %q: %w", npcType, err)
	}
	return &cfg, nil
}

func (s *MongoSource) LoadBTTree(treeName string) ([]byte, error) {
	data, ok := s.btTrees[treeName]
	if !ok {
		return nil, fmt.Errorf("config: BT tree %q not found in MongoDB", treeName)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: BT tree %q is not valid JSON", treeName)
	}
	return data, nil
}

func (s *MongoSource) LoadEventConfig(eventType string) ([]byte, error) {
	data, ok := s.eventTypes[eventType]
	if !ok {
		return nil, fmt.Errorf("config: event %q not found in MongoDB", eventType)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: event %q is not valid JSON", eventType)
	}
	return data, nil
}

func (s *MongoSource) LoadAllEventConfigs() (map[string][]byte, error) {
	result := make(map[string][]byte, len(s.eventTypes))
	for name, data := range s.eventTypes {
		result[name] = data
	}
	return result, nil
}

func (s *MongoSource) LoadNPCTemplate(name string) ([]byte, error) {
	data, ok := s.npcTemplates[name]
	if !ok {
		return nil, fmt.Errorf("config: NPC template %q not found in MongoDB", name)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC template %q is not valid JSON", name)
	}
	return data, nil
}

func (s *MongoSource) LoadNPCTypeConfig(npcType string) ([]byte, error) {
	data, ok := s.npcTypes[npcType]
	if !ok {
		return nil, fmt.Errorf("config: NPC type %q not found in MongoDB", npcType)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("config: NPC type %q is not valid JSON", npcType)
	}
	return data, nil
}

func (s *MongoSource) LoadAllNPCTemplates() (map[string][]byte, error) {
	result := make(map[string][]byte, len(s.npcTemplates))
	for name, data := range s.npcTemplates {
		result[name] = data
	}
	return result, nil
}

func (s *MongoSource) LoadRegionConfig(regionID string) ([]byte, error) {
	return nil, fmt.Errorf("config: region loading via MongoDB not yet implemented")
}

func (s *MongoSource) LoadAllRegionConfigs() (map[string][]byte, error) {
	return make(map[string][]byte), nil
}
