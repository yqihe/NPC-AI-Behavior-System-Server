// cmd/sync 从 ADMIN 平台 HTTP API 拉取全部配置，写入 configs/ 目录。
// 用途：联调后同步 ADMIN 侧新增/修改的配置到本地 JSON 文件，保持 e2e 测试与线上一致。
//
// 用法：
//
//	go run ./cmd/sync -api http://localhost:3000
//	go run ./cmd/sync -api http://localhost:3000 -out configs
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// httpConfigItem 与 ADMIN API 返回格式一致（复用 http_source 的结构）
type httpConfigItem struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type httpConfigResponse struct {
	Items []httpConfigItem `json:"items"`
}

// configCategory 定义一个配置类别及其对应的 API 路径和本地目录
type configCategory struct {
	name    string // 显示名
	apiPath string // ADMIN API endpoint
	dir     string // configs/ 下的子目录
}

var categories = []configCategory{
	{"event_types", "/api/configs/event_types", "events"},
	{"fsm_configs", "/api/configs/fsm_configs", "fsm"},
	{"bt_trees", "/api/configs/bt_trees", "bt_trees"},
	{"npc_templates", "/api/configs/npc_templates", "npc_templates"},
}

func main() {
	apiURL := flag.String("api", "", "ADMIN platform base URL (required), e.g. http://localhost:3000")
	outDir := flag.String("out", "configs", "output directory for JSON configs")
	flag.Parse()

	if *apiURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 去掉尾部斜杠
	base := strings.TrimRight(*apiURL, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := &http.Client{}
	var totalFiles int

	for _, cat := range categories {
		items, err := fetchItems(ctx, client, base+cat.apiPath)
		if err != nil {
			log.Fatalf("[%s] fetch failed: %v", cat.name, err)
		}

		written, err := writeItems(items, *outDir, cat.dir)
		if err != nil {
			log.Fatalf("[%s] write failed: %v", cat.name, err)
		}

		totalFiles += written
		log.Printf("[%s] synced %d configs", cat.name, written)
	}

	log.Printf("sync complete: %d files written to %s/", totalFiles, *outDir)
}

// fetchItems 从 ADMIN API 拉取一个类别的全部配置
func fetchItems(ctx context.Context, client *http.Client, url string) ([]httpConfigItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var r httpConfigResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return r.Items, nil
}

// writeItems 把配置条目写入本地 JSON 文件
// name 可含 "/" 如 "guard/idle"，自动创建子目录
func writeItems(items []httpConfigItem, outDir, subDir string) (int, error) {
	written := 0
	for _, item := range items {
		if item.Name == "" {
			continue
		}

		// 格式化 JSON（美化输出，方便 git diff）
		var pretty json.RawMessage
		formatted, err := json.MarshalIndent(json.RawMessage(item.Config), "", "  ")
		if err != nil {
			// 如果格式化失败，用原始数据
			pretty = item.Config
		} else {
			pretty = formatted
		}

		// 构建文件路径：outDir/subDir/name.json
		filePath := filepath.Join(outDir, subDir, item.Name+".json")

		// 确保目录存在（处理 "guard/idle" 这种含子目录的 name）
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("mkdir %s: %w", dir, err)
		}

		// 写入文件（末尾加换行符）
		content := append(pretty, '\n')
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return written, fmt.Errorf("write %s: %w", filePath, err)
		}

		written++
	}
	return written, nil
}
