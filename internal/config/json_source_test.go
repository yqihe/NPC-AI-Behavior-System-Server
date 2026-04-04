package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	// 从 internal/config/ 往上两层到项目根目录
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

func TestLoadFSMConfig_Civilian(t *testing.T) {
	root := projectRoot(t)
	src := config.NewJSONSource(filepath.Join(root, "configs"))

	cfg, err := src.LoadFSMConfig("civilian")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.InitialState != "Idle" {
		t.Fatalf("expected initial state Idle, got %s", cfg.InitialState)
	}
	if len(cfg.States) == 0 {
		t.Fatal("expected at least one state")
	}
	if len(cfg.Transitions) == 0 {
		t.Fatal("expected at least one transition")
	}
}

func TestLoadFSMConfig_NotFound(t *testing.T) {
	src := config.NewJSONSource("nonexistent_dir")
	_, err := src.LoadFSMConfig("civilian")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadBTTree_NotFound(t *testing.T) {
	src := config.NewJSONSource("nonexistent_dir")
	_, err := src.LoadBTTree("some_tree")
	if err == nil {
		t.Fatal("expected error for missing BT tree file")
	}
}

func TestLoadBTTree_InvalidJSON(t *testing.T) {
	// 创建临时目录和无效 JSON 文件
	tmpDir := t.TempDir()
	btDir := filepath.Join(tmpDir, "bt_trees")
	os.MkdirAll(btDir, 0755)
	os.WriteFile(filepath.Join(btDir, "bad.json"), []byte(`{invalid`), 0644)

	src := config.NewJSONSource(tmpDir)
	_, err := src.LoadBTTree("bad")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- LoadEventConfig ---

func TestLoadEventConfig_Explosion(t *testing.T) {
	root := projectRoot(t)
	src := config.NewJSONSource(filepath.Join(root, "configs"))

	data, err := src.LoadEventConfig("explosion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}
}

func TestLoadEventConfig_NotFound(t *testing.T) {
	src := config.NewJSONSource("nonexistent_dir")
	_, err := src.LoadEventConfig("explosion")
	if err == nil {
		t.Fatal("expected error for missing event config")
	}
}

func TestLoadEventConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	evtDir := filepath.Join(tmpDir, "events")
	os.MkdirAll(evtDir, 0755)
	os.WriteFile(filepath.Join(evtDir, "bad.json"), []byte(`{invalid`), 0644)

	src := config.NewJSONSource(tmpDir)
	_, err := src.LoadEventConfig("bad")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- LoadAllEventConfigs ---

func TestLoadAllEventConfigs(t *testing.T) {
	root := projectRoot(t)
	src := config.NewJSONSource(filepath.Join(root, "configs"))

	result, err := src.LoadAllEventConfigs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one event config")
	}
	if _, ok := result["explosion"]; !ok {
		t.Fatal("expected explosion config in results")
	}
}

func TestLoadAllEventConfigs_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	evtDir := filepath.Join(tmpDir, "events")
	os.MkdirAll(evtDir, 0755)

	src := config.NewJSONSource(tmpDir)
	result, err := src.LoadAllEventConfigs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(result))
	}
}

func TestLoadAllEventConfigs_DirNotExist(t *testing.T) {
	src := config.NewJSONSource("nonexistent_dir")
	_, err := src.LoadAllEventConfigs()
	if err == nil {
		t.Fatal("expected error for missing events dir")
	}
}

// --- LoadNPCTypeConfig ---

func TestLoadNPCTypeConfig_Civilian(t *testing.T) {
	root := projectRoot(t)
	src := config.NewJSONSource(filepath.Join(root, "configs"))

	data, err := src.LoadNPCTypeConfig("civilian")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}
}

func TestLoadNPCTypeConfig_NotFound(t *testing.T) {
	src := config.NewJSONSource("nonexistent_dir")
	_, err := src.LoadNPCTypeConfig("civilian")
	if err == nil {
		t.Fatal("expected error for missing NPC type config")
	}
}
