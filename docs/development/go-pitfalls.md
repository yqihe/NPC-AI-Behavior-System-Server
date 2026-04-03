# Go 常见陷阱

编写代码时主动检查的清单。在开发过程中踩到新坑时追加到本文档。

## 并发

- **共享状态必须加锁**：多个 goroutine 读写同一个 map/slice/struct 字段时，必须用 `sync.RWMutex` 或 channel 保护。Blackboard 是重灾区
- **goroutine 泄漏**：启动的 goroutine 必须有退出路径，通过 `context.Context` 取消或 channel 关闭。`time.AfterFunc` 回调里引用的对象不会被 GC
- **defer 在循环里**：`for` 循环内的 `defer` 不会在每次迭代结束时执行，而是在函数返回时。循环内需要释放资源时手动关闭或提取为子函数
- **闭包捕获循环变量**：`for i, v := range` 的 `v` 在 goroutine 闭包中会捕获引用而非值（Go 1.22 前）。传参或重新赋值
- **sync.WaitGroup Add 必须在 goroutine 外**：`wg.Add(1)` 必须在 `go func()` 之前调用，否则 `wg.Wait()` 可能提前返回
- **channel 死锁**：无缓冲 channel 的发送和接收必须在不同 goroutine，否则死锁。关闭已关闭的 channel 会 panic
- **channel 发送方必须考虑接收方生命周期**：如果接收方 goroutine 可能已退出（如 ctx 取消），发送方的阻塞写入会永久挂起。用 `select + default` 非阻塞发送或 `select + ctx.Done()` 保护

## 数据结构

- **nil map 写入 panic**：`var m map[string]int; m["a"] = 1` 直接 panic。必须 `make(map[string]int)` 初始化
- **nil slice 可以 append**：`var s []int; s = append(s, 1)` 是安全的，但直接 `s[0] = 1` 会 panic
- **nil slice JSON 序列化为 null**：`var s []int` 序列化为 `null`，而 `s := make([]int, 0)` 序列化为 `[]`。面向客户端的 API 响应中，必须用 `make` 初始化确保序列化为空数组
- **map 不是并发安全的**：多 goroutine 读写 map 会 fatal error（不是 panic，recover 不了）。用 `sync.RWMutex` 或 `sync.Map`
- **slice 扩容后底层数组变化**：append 可能返回新的底层数组，之前持有的引用指向旧数据

## 接口

- **typed nil vs nil interface**：`var p *MyStruct = nil; var i interface{} = p; i != nil` 是 true。接口值只有类型和值都为 nil 时才等于 nil
- **空接口断言失败**：`val.(int)` 如果 val 不是 int 会 panic。用 `val, ok := val.(int)` 两值形式

## 错误处理

- **error 不要忽略**：`f, _ := os.Open(path)` 后面用 f 会 nil pointer panic。除非你确定不会出错且写了注释说明原因
- **errors.Is / errors.As**：比较 error 用 `errors.Is`，不用 `==`。提取具体类型用 `errors.As`，不用类型断言
- **defer Close 的 error**：`defer f.Close()` 会吞掉 Close 的 error。写文件时需要 `defer func() { err = f.Close() }()` 或在 defer 前显式检查

## 性能

- **json.Unmarshal 到 any 丢失 int/float 区分**：Go 的 `json.Unmarshal` 把所有 JSON 数字解析为 `float64`。存入 MongoDB 后 `priority: 10` 变成 BSON double `10.0`，再转回 JSON 时无法反序列化为 Go 的 `int` 字段。解决：用 `bson.UnmarshalExtJSON` 直接从 JSON 转 BSON Raw
- **string 和 []byte 转换有拷贝**：频繁转换时考虑 `unsafe` 或 `strings.Builder`
- **fmt.Sprintf 在热路径上很慢**：Tick 循环内避免用 Sprintf 拼日志 key，用预定义常量
- **time.After 在循环里会泄漏**：每次调用创建一个 timer 不会被回收直到触发。用 `time.NewTimer` + `Reset`

## 测试

- **测试里的 goroutine panic 不会导致测试失败**：会直接崩掉整个进程。测试中的 goroutine 里用 `t.Error` 而非 `panic`
- **-race 标志**：并发代码必须用 `go test -race` 检测竞态。CI 里默认开启

---

*在开发过程中踩到新坑时追加到本文档对应分类下。*
