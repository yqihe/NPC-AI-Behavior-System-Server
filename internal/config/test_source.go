package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewSourceFromDir 从目录加载配置到内存（仅用于测试），目录结构需与 ADMIN API 一致：
//
//	dir/fsm/*.json
//	dir/bt_trees/**/*.json
//	dir/events/*.json
//	dir/npc_types/*.json
func NewSourceFromDir(dir string) (Source, error) {
	s := &HTTPSource{
		npcTypes:   make(map[string][]byte),
		fsmConfigs: make(map[string][]byte),
		btTrees:    make(map[string][]byte),
		eventTypes: make(map[string][]byte),
	}

	loaders := []struct {
		subdir string
		target map[string][]byte
	}{
		{"fsm", s.fsmConfigs},
		{"events", s.eventTypes},
		{"npc_types", s.npcTypes},
	}

	for _, l := range loaders {
		if err := loadFlatDir(filepath.Join(dir, l.subdir), l.target); err != nil {
			return nil, err
		}
	}

	// bt_trees 支持子目录（如 civilian/idle）
	if err := loadRecursiveDir(filepath.Join(dir, "bt_trees"), s.btTrees); err != nil {
		return nil, err
	}

	return s, nil
}

func loadFlatDir(dir string, target map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("config: read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		target[name] = data
	}
	return nil
}

func loadRecursiveDir(baseDir string, target map[string][]byte) error {
	return filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		rel, _ := filepath.Rel(baseDir, path)
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		target[name] = data
		return nil
	})
}
