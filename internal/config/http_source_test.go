package config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
)

// newTestAdminServer 创建模拟 ADMIN API 的 httptest.Server
// 返回包含完整配置数据的 4 个 endpoint
func newTestAdminServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/configs/event_types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"explosion","config":{"name":"explosion","default_severity":80,"default_ttl":15.0,"perception_mode":"auditory","range":500.0}}]}`)
	})

	mux.HandleFunc("/api/configs/npc_templates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"template_ref":"admin-uuid-1","fields":{"hp":100,"visual_range":200.0,"auditory_range":500.0},"behavior":{"fsm_ref":"civilian","bt_refs":{"Idle":"civilian/idle"}}}}]}`)
	})

	mux.HandleFunc("/api/configs/fsm_configs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"initial_state":"Idle","states":[{"name":"Idle"},{"name":"Alarmed"}],"transitions":[{"from":"Idle","to":"Alarmed","priority":10,"condition":{"key":"last_event_type","op":"!=","value":""}}]}}]}`)
	})

	mux.HandleFunc("/api/configs/bt_trees", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"civilian/idle","config":{"type":"stub_action","params":{"name":"idle_wander","result":"success"}}}]}`)
	})

	mux.HandleFunc("/api/configs/regions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"meadow","config":{"region_id":"meadow","name":"风语草原","region_type":"wilderness","spawn_table":[{"template_ref":"civilian","count":2,"spawn_points":[{"x":10,"z":20}],"wander_radius":30.0,"respawn_seconds":120.0}]}}]}`)
	})

	return httptest.NewServer(mux)
}

func TestHTTPSource_LoadAll(t *testing.T) {
	srv := newTestAdminServer()
	defer srv.Close()

	ctx := context.Background()
	src, err := config.NewHTTPSource(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPSource: %v", err)
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

	// LoadNPCTemplate (ADMIN v3 shape)
	tmplData, err := src.LoadNPCTemplate("civilian")
	if err != nil {
		t.Fatalf("LoadNPCTemplate: %v", err)
	}
	if !json.Valid(tmplData) {
		t.Error("LoadNPCTemplate returned invalid JSON")
	}

	// LoadAllNPCTemplates
	allTmpls, err := src.LoadAllNPCTemplates()
	if err != nil {
		t.Fatalf("LoadAllNPCTemplates: %v", err)
	}
	if len(allTmpls) != 1 {
		t.Errorf("expected 1 NPC template, got %d", len(allTmpls))
	}
	if _, ok := allTmpls["civilian"]; !ok {
		t.Error("civilian not found in LoadAllNPCTemplates")
	}
}

func TestHTTPSource_NotFound(t *testing.T) {
	srv := newTestAdminServer()
	defer srv.Close()

	ctx := context.Background()
	src, err := config.NewHTTPSource(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPSource: %v", err)
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
	if _, err := src.LoadNPCTemplate("nonexistent"); err == nil {
		t.Error("expected error for nonexistent NPC template")
	}
}

func TestHTTPSource_DisconnectAfterLoad(t *testing.T) {
	srv := newTestAdminServer()

	ctx := context.Background()
	src, err := config.NewHTTPSource(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPSource: %v", err)
	}

	// 关闭 server，模拟 ADMIN 不可达
	srv.Close()

	// 读取应该仍然正常——纯内存
	fsmCfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatalf("LoadFSMConfig after server close: %v", err)
	}
	if fsmCfg.InitialState != "Idle" {
		t.Errorf("expected Idle, got %s", fsmCfg.InitialState)
	}

	btData, err := src.LoadBTTree("civilian/idle")
	if err != nil {
		t.Fatalf("LoadBTTree after server close: %v", err)
	}
	if !json.Valid(btData) {
		t.Error("invalid JSON after server close")
	}
}

func TestHTTPSource_Unreachable(t *testing.T) {
	ctx := context.Background()
	_, err := config.NewHTTPSource(ctx, "http://127.0.0.1:19999")
	if err == nil {
		t.Fatal("expected error for unreachable ADMIN API")
	}
}

func TestHTTPSource_EmptyItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	_, err := config.NewHTTPSource(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestHTTPSource_Non200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	_, err := config.NewHTTPSource(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestHTTPSource_Regions_Happy(t *testing.T) {
	srv := newTestAdminServer()
	defer srv.Close()

	ctx := context.Background()
	src, err := config.NewHTTPSource(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPSource: %v", err)
	}

	data, err := src.LoadRegionConfig("meadow")
	if err != nil {
		t.Fatalf("LoadRegionConfig(meadow): %v", err)
	}
	var z struct {
		RegionID   string `json:"region_id"`
		Name       string `json:"name"`
		RegionType string `json:"region_type"`
	}
	if err := json.Unmarshal(data, &z); err != nil {
		t.Fatalf("unmarshal region: %v", err)
	}
	if z.RegionID != "meadow" {
		t.Errorf("expected region_id=meadow, got %q", z.RegionID)
	}
	if z.Name != "风语草原" {
		t.Errorf("expected display name=风语草原, got %q", z.Name)
	}

	all, err := src.LoadAllRegionConfigs()
	if err != nil {
		t.Fatalf("LoadAllRegionConfigs: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 region, got %d", len(all))
	}
	if _, ok := all["meadow"]; !ok {
		t.Error("meadow not in LoadAllRegionConfigs")
	}

	if _, err := src.LoadRegionConfig("nonexistent"); err == nil {
		t.Error("expected error for nonexistent region")
	}
}

// TestHTTPSource_Regions_EmptyItems 空 items[] 合法（与 JSONSource 目录不存在行为一致）
func TestHTTPSource_Regions_EmptyItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/configs/event_types", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"explosion","config":{"name":"explosion","default_severity":80,"default_ttl":15.0,"perception_mode":"auditory","range":500.0}}]}`)
	})
	mux.HandleFunc("/api/configs/npc_templates", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"template_ref":"admin-uuid-1","fields":{"hp":100,"visual_range":200.0,"auditory_range":500.0},"behavior":{"fsm_ref":"civilian","bt_refs":{"Idle":"civilian/idle"}}}}]}`)
	})
	mux.HandleFunc("/api/configs/fsm_configs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"initial_state":"Idle","states":[{"name":"Idle"}],"transitions":[]}}]}`)
	})
	mux.HandleFunc("/api/configs/bt_trees", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian/idle","config":{"type":"stub_action","params":{"name":"idle","result":"success"}}}]}`)
	})
	mux.HandleFunc("/api/configs/regions", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	src, err := config.NewHTTPSource(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPSource with empty regions: %v", err)
	}
	all, err := src.LoadAllRegionConfigs()
	if err != nil {
		t.Fatalf("LoadAllRegionConfigs: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 regions, got %d", len(all))
	}
}

// TestHTTPSource_Regions_Dangling 500 + 业务码 47011：提取 details[] 悬空引用 fail-fast
func TestHTTPSource_Regions_Dangling(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/configs/event_types", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"explosion","config":{"name":"explosion","default_severity":80,"default_ttl":15.0,"perception_mode":"auditory","range":500.0}}]}`)
	})
	mux.HandleFunc("/api/configs/npc_templates", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"template_ref":"admin-uuid-1","fields":{"hp":100,"visual_range":200.0,"auditory_range":500.0},"behavior":{"fsm_ref":"civilian","bt_refs":{"Idle":"civilian/idle"}}}}]}`)
	})
	mux.HandleFunc("/api/configs/fsm_configs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian","config":{"initial_state":"Idle","states":[{"name":"Idle"}],"transitions":[]}}]}`)
	})
	mux.HandleFunc("/api/configs/bt_trees", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"name":"civilian/idle","config":{"type":"stub_action","params":{"name":"idle","result":"success"}}}]}`)
	})
	mux.HandleFunc("/api/configs/regions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{
			"code": 47011,
			"message": "区域导出失败：存在悬空的 NPC 模板引用",
			"details": [
				{"npc_name": "village_outskirts", "ref_type": "npc_template_ref", "ref_value": "villager_guard", "reason": "missing_or_disabled"}
			]
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	_, err := config.NewHTTPSource(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected fail-fast on code=47011")
	}
	msg := err.Error()
	if !strings.Contains(msg, "47011") || !strings.Contains(msg, "dangling") {
		t.Errorf("expected err to mention 47011 and dangling, got %q", msg)
	}
}

func TestHTTPSource_Timeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 阻塞直到请求被取消
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 用一个很短的 context 超时触发
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := config.NewHTTPSource(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error for timeout")
	}
}
