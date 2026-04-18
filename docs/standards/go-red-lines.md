# Go 语言红线

Go 代码的绝对禁令。违反任何一条都是 bug，不是风格问题。
经验性陷阱和最佳实践见 `go-pitfalls.md`。

## 资源泄漏

- **禁止**不关闭外部连接。`http.Response.Body`、`mongo.Client` 等必须在 `defer` 或 graceful shutdown 中关闭
- **禁止**用查询 context 关闭 cursor。查询 context 可能已超时，cursor.Close 会失败导致泄漏。用 `context.Background()`
- **禁止**裸 `context.Background()` 做外部 IO。所有网络请求、外部调用必须带 timeout context
- **禁止**多个 goroutine close 同一 channel。用单一 owner 或 `sync.Once` 保护

## 序列化

- **禁止** nil slice 面向客户端。`var s []T` 序列化为 `null`，必须 `make([]T, 0)` 确保输出 `[]`
- **禁止** nil map 面向客户端。`var m map[string]T` 序列化为 `null`，必须 `make(map[string]T)` 确保输出 `{}`
- **禁止**对合法零值字段用 `omitempty`。`json:"severity,omitempty"` 会丢弃 `0`，而 `0` 可能是有效业务值
- **禁止** `json.Unmarshal` 到 `any` 后假设 int 类型。JSON 数字一律解析为 `float64`

## 错误处理

- **禁止** writeError / http.Error 后不 return。不 return 会继续执行后续代码，可能二次写入或空指针 panic
- **禁止**返回 typed nil 作为 error。`var e *MyError = nil; return e` 导致 `err != nil` 为 true。直接 `return nil`
- **禁止**在 API 响应中暴露 Go error 字符串。使用预定义错误描述，原始 error 写日志

## 字符串

- **禁止**用 `len()` 计算中文字符串字符数。`len("中文")` = 6 字节。用 `utf8.RuneCountInString()`

## 错误码

- **禁止**同一错误码用于不同语义。每个错误码必须唯一映射到一个含义

## 魔法字符串

- **禁止**在多处使用同一字符串字面量。复用的字符串必须定义为常量

## Graceful Shutdown

- **禁止**反序关闭。必须：停止接受请求 → 等待进行中请求完成 → 关闭外部连接。反了会导致请求写入已关闭的连接
