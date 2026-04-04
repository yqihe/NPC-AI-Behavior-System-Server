# Go 语言陷阱与禁令

编写 Go 代码时主动检查的清单。踩到新坑时追加。

## 并发

- **共享状态必须加锁**：多 goroutine 读写同一 map/slice/struct 字段必须用 `sync.RWMutex` 或 channel
- **goroutine 泄漏**：启动的 goroutine 必须有退出路径（`context.Context` 取消或 channel 关闭）
- **defer 在循环里**：`for` 循环内的 `defer` 不会在每次迭代结束时执行。循环内需要释放资源时手动关闭或提取为子函数
- **闭包捕获循环变量**：`for i, v := range` 的 `v` 在 goroutine 闭包中捕获引用而非值（Go 1.22 前）
- **sync.WaitGroup Add 必须在 goroutine 外**：`wg.Add(1)` 必须在 `go func()` 之前调用
- **channel 死锁**：无缓冲 channel 的发送和接收必须在不同 goroutine。关闭已关闭的 channel 会 panic
- **channel 发送方必须考虑接收方生命周期**：接收方已退出时，阻塞写入会永久挂起。用 `select + ctx.Done()` 保护

## 数据结构

- **nil map 写入 panic**：`var m map[string]int; m["a"] = 1` 直接 panic。必须 `make()` 初始化
- **nil slice 可以 append 但不能索引**：`var s []int; s = append(s, 1)` 安全，`s[0] = 1` panic
- **nil slice JSON 序列化为 null**：面向客户端的响应必须用 `make([]T, 0)` 初始化，确保序列化为 `[]`
- **map 不是并发安全的**：多 goroutine 读写 map 会 fatal error（recover 不了）
- **slice 扩容后底层数组变化**：append 可能返回新底层数组，之前持有的引用指向旧数据

## 接口

- **typed nil vs nil interface**：`var p *MyStruct = nil; var i interface{} = p; i != nil` 是 true
- **空接口断言失败 panic**：`val.(int)` 如果 val 不是 int 会 panic。用 `val, ok := val.(int)` 两值形式

## 错误处理

- **禁止忽略 error**：`f, _ := os.Open(path)` 后面用 f 会 nil pointer panic。除非确定不会出错且写了注释
- **禁止忽略 json.Unmarshal error**：测试和生产代码中都必须检查。忽略会让格式问题静默通过
- **errors.Is / errors.As**：比较 error 用 `errors.Is`，提取具体类型用 `errors.As`
- **defer Close 的 error**：写文件时需要 `defer func() { err = f.Close() }()` 或显式检查

## JSON / BSON

- **json.Unmarshal 到 any 丢失 int**：所有 JSON 数字解析为 `float64`。存入 MongoDB 后 `priority: 10` 变成 double `10.0`，再转回 Go `int` 字段失败
- **omitempty 吞零值**：`json:"severity,omitempty"` 把 `0` 丢弃。`0` 是合法业务值时禁止用 omitempty
- **bson tag 漏写**：只写 `json` tag 没写 `bson` tag，MongoDB 字段名变大写开头，读回来映射不上

## HTTP Handler

- **writeError 后必须 return**：不 return 会继续执行后续代码，可能二次写入或空指针 panic

## 性能

- **string 和 []byte 转换有拷贝**：频繁转换时考虑 `strings.Builder`
- **fmt.Sprintf 在热路径上很慢**：Tick 循环内用预定义常量
- **time.After 在循环里泄漏**：每次调用创建 timer 不会被回收。用 `time.NewTimer` + `Reset`

## 测试

- **测试中 goroutine panic 崩掉整个进程**：用 `t.Error` 而非 `panic`
- **-race 标志**：并发代码必须用 `go test -race` 检测竞态
- **禁止硬编码依赖配置文件的计数**：如 `expected 4 transitions` 会因配置变更假阳性失败
