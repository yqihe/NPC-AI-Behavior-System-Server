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
	if len(cfg.States) != 5 {
		t.Fatalf("expected 5 states, got %d", len(cfg.States))
	}
	if len(cfg.Transitions) != 4 {
		t.Fatalf("expected 4 transitions, got %d", len(cfg.Transitions))
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
