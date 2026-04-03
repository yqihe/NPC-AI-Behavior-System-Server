package config_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
)

const (
	testMongoURI = "mongodb://localhost:27017"
	testDB       = "npc_ai_test"
)

// skipIfNoMongo 无 MongoDB 时跳过测试
func skipIfNoMongo(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping MongoDB test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI))
	if err != nil {
		t.Skipf("skipping: cannot connect to MongoDB: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		t.Skipf("skipping: MongoDB not reachable: %v", err)
	}
	client.Disconnect(ctx)
}

// seedTestData 向测试数据库插入测试配置
func seedTestData(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI))
	if err != nil {
		t.Fatal(err)
	}

	db := client.Database(testDB)
	// 清空
	db.Drop(ctx)

	// event_types
	db.Collection("event_types").InsertOne(ctx, bson.M{
		"name": "explosion",
		"config": bson.M{
			"name":             "explosion",
			"default_severity": 80,
			"default_ttl":      15.0,
			"perception_mode":  "auditory",
			"range":            500.0,
		},
	})

	// npc_types
	db.Collection("npc_types").InsertOne(ctx, bson.M{
		"name": "civilian",
		"config": bson.M{
			"type_name": "civilian",
			"fsm_ref":   "civilian",
			"bt_refs":   bson.M{"Idle": "civilian/idle"},
			"perception": bson.M{
				"visual_range":   200.0,
				"auditory_range": 500.0,
			},
		},
	})

	// fsm_configs
	db.Collection("fsm_configs").InsertOne(ctx, bson.M{
		"name": "civilian",
		"config": bson.M{
			"initial_state": "Idle",
			"states":        bson.A{bson.M{"name": "Idle"}, bson.M{"name": "Alarmed"}},
			"transitions": bson.A{bson.M{
				"from": "Idle", "to": "Alarmed", "priority": 10,
				"condition": bson.M{"key": "last_event_type", "op": "!=", "value": ""},
			}},
		},
	})

	// bt_trees
	db.Collection("bt_trees").InsertOne(ctx, bson.M{
		"name": "civilian/idle",
		"config": bson.M{
			"type":   "stub_action",
			"params": bson.M{"name": "idle_wander", "result": "success"},
		},
	})

	cleanup := func() {
		db.Drop(ctx)
		client.Disconnect(ctx)
	}
	return cleanup
}

func TestMongoSource_LoadAll(t *testing.T) {
	skipIfNoMongo(t)
	cleanup := seedTestData(t)
	defer cleanup()

	ctx := context.Background()
	src, err := config.NewMongoSource(ctx, testMongoURI, testDB)
	if err != nil {
		t.Fatalf("NewMongoSource: %v", err)
	}

	// LoadFSMConfig
	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatalf("LoadFSMConfig: %v", err)
	}
	if fsmCfg.InitialState != "Idle" {
		t.Errorf("expected InitialState=Idle, got %s", fsmCfg.InitialState)
	}

	// LoadBTTree
	btData, err := src.LoadBTTree("civilian/idle")
	if err != nil {
		t.Fatalf("LoadBTTree: %v", err)
	}
	if !json.Valid(btData) {
		t.Error("LoadBTTree returned invalid JSON")
	}

	// LoadEventConfig
	evtData, err := src.LoadEventConfig("explosion")
	if err != nil {
		t.Fatalf("LoadEventConfig: %v", err)
	}
	if !json.Valid(evtData) {
		t.Error("LoadEventConfig returned invalid JSON")
	}

	// LoadAllEventConfigs
	allEvts, err := src.LoadAllEventConfigs()
	if err != nil {
		t.Fatalf("LoadAllEventConfigs: %v", err)
	}
	if len(allEvts) != 1 {
		t.Errorf("expected 1 event config, got %d", len(allEvts))
	}
	if _, ok := allEvts["explosion"]; !ok {
		t.Error("explosion not found in LoadAllEventConfigs")
	}

	// LoadNPCTypeConfig
	npcData, err := src.LoadNPCTypeConfig("civilian")
	if err != nil {
		t.Fatalf("LoadNPCTypeConfig: %v", err)
	}
	if !json.Valid(npcData) {
		t.Error("LoadNPCTypeConfig returned invalid JSON")
	}
}

func TestMongoSource_NotFound(t *testing.T) {
	skipIfNoMongo(t)
	cleanup := seedTestData(t)
	defer cleanup()

	ctx := context.Background()
	src, err := config.NewMongoSource(ctx, testMongoURI, testDB)
	if err != nil {
		t.Fatalf("NewMongoSource: %v", err)
	}

	if _, err := src.LoadFSMConfig("nonexistent"); err == nil {
		t.Error("expected error for nonexistent FSM config")
	}
	if _, err := src.LoadBTTree("nonexistent"); err == nil {
		t.Error("expected error for nonexistent BT tree")
	}
	if _, err := src.LoadEventConfig("nonexistent"); err == nil {
		t.Error("expected error for nonexistent event config")
	}
	if _, err := src.LoadNPCTypeConfig("nonexistent"); err == nil {
		t.Error("expected error for nonexistent NPC type config")
	}
}

func TestMongoSource_ConnectError(t *testing.T) {
	skipIfNoMongo(t) // 需要 Docker 环境，但用错误 URI 测试

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := config.NewMongoSource(ctx, "mongodb://localhost:19999", testDB)
	if err == nil {
		t.Fatal("expected error for bad MongoDB URI")
	}
}

func TestMongoSource_MemoryOnly(t *testing.T) {
	skipIfNoMongo(t)
	cleanup := seedTestData(t)
	defer cleanup()

	ctx := context.Background()
	src, err := config.NewMongoSource(ctx, testMongoURI, testDB)
	if err != nil {
		t.Fatalf("NewMongoSource: %v", err)
	}

	// NewMongoSource 已断开连接（defer Disconnect）
	// 读取应该仍然正常——纯内存
	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatalf("LoadFSMConfig after disconnect: %v", err)
	}
	if fsmCfg.InitialState != "Idle" {
		t.Errorf("expected Idle, got %s", fsmCfg.InitialState)
	}

	btData, err := src.LoadBTTree("civilian/idle")
	if err != nil {
		t.Fatalf("LoadBTTree after disconnect: %v", err)
	}
	if !json.Valid(btData) {
		t.Error("invalid JSON after disconnect")
	}
}
