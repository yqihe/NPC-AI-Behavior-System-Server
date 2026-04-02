package bt

import (
	"encoding/json"
	"fmt"
)

// --- check_bb_float ---

type checkBBFloat struct {
	key   string
	op    string
	value float64
}

func (c *checkBBFloat) Tick(ctx *Context) Status {
	val, ok := ctx.BB.GetRaw(c.key)
	if !ok {
		return Failure
	}
	f, isNum := toFloat64(val)
	if !isNum {
		return Failure
	}
	if compareFloat64(f, c.op, c.value) {
		return Success
	}
	return Failure
}

func checkBBFloatFactory(params json.RawMessage) (Node, error) {
	var cfg struct {
		Key   string  `json:"key"`
		Op    string  `json:"op"`
		Value float64 `json:"value"`
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return nil, fmt.Errorf("check_bb_float: %w", err)
	}
	if cfg.Key == "" || cfg.Op == "" {
		return nil, fmt.Errorf("check_bb_float: key and op are required")
	}
	return &checkBBFloat{key: cfg.Key, op: cfg.Op, value: cfg.Value}, nil
}

// --- check_bb_string ---

type checkBBString struct {
	key   string
	op    string
	value string
}

func (c *checkBBString) Tick(ctx *Context) Status {
	val, ok := ctx.BB.GetRaw(c.key)
	if !ok {
		return Failure
	}
	s, isStr := val.(string)
	if !isStr {
		return Failure
	}
	if compareString(s, c.op, c.value) {
		return Success
	}
	return Failure
}

func checkBBStringFactory(params json.RawMessage) (Node, error) {
	var cfg struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return nil, fmt.Errorf("check_bb_string: %w", err)
	}
	if cfg.Key == "" || cfg.Op == "" {
		return nil, fmt.Errorf("check_bb_string: key and op are required")
	}
	return &checkBBString{key: cfg.Key, op: cfg.Op, value: cfg.Value}, nil
}

// --- set_bb_value ---

type setBBValue struct {
	key   string
	value any
}

func (s *setBBValue) Tick(ctx *Context) Status {
	ctx.BB.SetRaw(s.key, s.value)
	return Success
}

func setBBValueFactory(params json.RawMessage) (Node, error) {
	var cfg struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return nil, fmt.Errorf("set_bb_value: %w", err)
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("set_bb_value: key is required")
	}
	return &setBBValue{key: cfg.Key, value: cfg.Value}, nil
}

// --- 内部比较辅助 ---

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func compareFloat64(a float64, op string, b float64) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	default:
		return false
	}
}

func compareString(a string, op string, b string) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	default:
		return false
	}
}
