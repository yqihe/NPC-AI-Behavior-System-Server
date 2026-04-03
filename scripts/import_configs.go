// 配置导入脚本：将 configs/ 目录下的 JSON 文件导入 MongoDB
//
// 用法：
//   go run scripts/import_configs.go -uri=mongodb://localhost:27017 -db=npc_ai -dir=configs

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	uri := flag.String("uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db", "npc_ai", "MongoDB database name")
	dir := flag.String("dir", "configs", "configs directory path")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(*uri))
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "ping error: %v\n", err)
		os.Exit(1)
	}

	db := client.Database(*dbName)
	fmt.Printf("connected to %s, database: %s\n", *uri, *dbName)

	// 导入 4 类配置
	importFlat(ctx, db, filepath.Join(*dir, "events"), "event_types")
	importFlat(ctx, db, filepath.Join(*dir, "npc_types"), "npc_types")
	importFlat(ctx, db, filepath.Join(*dir, "fsm"), "fsm_configs")
	importRecursive(ctx, db, filepath.Join(*dir, "bt_trees"), "bt_trees")

	fmt.Println("done")
}

// importFlat 导入单层目录下的 JSON 文件
// 文件名（不含 .json）作为 name
func importFlat(ctx context.Context, db *mongo.Database, dir, collection string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir %s: %v\n", dir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		path := filepath.Join(dir, entry.Name())
		upsertConfig(ctx, db, collection, name, path)
	}
}

// importRecursive 递归导入目录下的 JSON 文件
// 相对于 baseDir 的路径（不含 .json）作为 name，如 "civilian/idle"
func importRecursive(ctx context.Context, db *mongo.Database, baseDir, collection string) {
	filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		// 计算相对于 baseDir 的路径作为 name
		rel, _ := filepath.Rel(baseDir, path)
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		upsertConfig(ctx, db, collection, name, path)
		return nil
	})
}

// upsertConfig 读取 JSON 文件并 upsert 到 MongoDB
func upsertConfig(ctx context.Context, db *mongo.Database, collection, name, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] %s/%s: read: %v\n", collection, name, err)
		return
	}

	// JSON → BSON Raw（保留整数/浮点类型区分）
	if !json.Valid(data) {
		fmt.Fprintf(os.Stderr, "  [FAIL] %s/%s: invalid JSON\n", collection, name)
		return
	}
	var configRaw bson.Raw
	if err := bson.UnmarshalExtJSON(data, false, &configRaw); err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] %s/%s: json→bson: %v\n", collection, name, err)
		return
	}

	coll := db.Collection(collection)
	filter := bson.M{"name": name}
	update := bson.M{"$set": bson.M{"name": name, "config": configRaw}}
	opts := options.UpdateOne().SetUpsert(true)

	result, err := coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [FAIL] %s/%s: upsert: %v\n", collection, name, err)
		return
	}

	if result.UpsertedCount > 0 {
		fmt.Printf("  [INSERT] %s/%s\n", collection, name)
	} else {
		fmt.Printf("  [UPDATE] %s/%s\n", collection, name)
	}
}
