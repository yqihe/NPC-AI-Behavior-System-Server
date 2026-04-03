package rule

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// Condition 是条件树的节点。
// 叶子节点包含 Key+Op+Value/RefKey，组合节点包含 And/Or 子条件。
type Condition struct {
	// 叶子节点字段
	Key    string          `json:"key,omitempty"`
	Op     string          `json:"op,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"`
	RefKey string          `json:"ref_key,omitempty"`

	// 组合节点字段
	And []Condition `json:"and,omitempty"`
	Or  []Condition `json:"or,omitempty"`
}

// IsEmpty 判断条件是否为空（无条件转换，始终为 true）
func (c *Condition) IsEmpty() bool {
	return c.Key == "" && len(c.And) == 0 && len(c.Or) == 0
}

// isLeaf 判断是否是叶子节点
func (c *Condition) isLeaf() bool {
	return c.Key != ""
}

// Evaluate 对 Blackboard 求值，返回条件是否满足
func (c *Condition) Evaluate(bb *blackboard.Blackboard) bool {
	// 空条件 → 始终 true
	if c.IsEmpty() {
		return true
	}

	// AND 组合
	if len(c.And) > 0 {
		for i := range c.And {
			if !c.And[i].Evaluate(bb) {
				return false
			}
		}
		return true
	}

	// OR 组合
	if len(c.Or) > 0 {
		for i := range c.Or {
			if c.Or[i].Evaluate(bb) {
				return true
			}
		}
		return false
	}

	// 叶子节点
	if c.isLeaf() {
		return c.evaluateLeaf(bb)
	}

	return true
}

// evaluateLeaf 求值叶子节点
func (c *Condition) evaluateLeaf(bb *blackboard.Blackboard) bool {
	bbVal, ok := bb.GetRaw(c.Key)
	if !ok {
		return false
	}

	// 确定比较目标值
	var target any
	if c.RefKey != "" {
		refVal, refOk := bb.GetRaw(c.RefKey)
		if !refOk {
			return false
		}
		target = refVal
	} else {
		target = decodeJSONValue(c.Value)
	}

	return compare(bbVal, c.Op, target)
}

// Validate 校验条件中引用的所有 Key 是否在 Blackboard 注册表中
func (c *Condition) Validate() error {
	if c.IsEmpty() {
		return nil
	}

	// 禁止同时有 leaf（key）和 composite（and/or）字段
	if c.isLeaf() && (len(c.And) > 0 || len(c.Or) > 0) {
		return fmt.Errorf("condition cannot have both key %q and and/or composite fields", c.Key)
	}

	if len(c.And) > 0 {
		for i := range c.And {
			if err := c.And[i].Validate(); err != nil {
				return err
			}
		}
		return nil
	}

	if len(c.Or) > 0 {
		for i := range c.Or {
			if err := c.Or[i].Validate(); err != nil {
				return err
			}
		}
		return nil
	}

	if c.isLeaf() {
		if err := blackboard.ValidateKeyName(c.Key); err != nil {
			return fmt.Errorf("condition key: %w", err)
		}
		if c.RefKey != "" {
			if err := blackboard.ValidateKeyName(c.RefKey); err != nil {
				return fmt.Errorf("condition ref_key: %w", err)
			}
		}
		if err := validateOp(c.Op); err != nil {
			return err
		}
	}

	return nil
}

// --- 操作符 ---

var validOps = map[string]bool{
	"==": true, "!=": true,
	">": true, ">=": true,
	"<": true, "<=": true,
	"in": true,
}

func validateOp(op string) error {
	if !validOps[op] {
		return fmt.Errorf("condition: unknown operator %q, supported: %s",
			op, strings.Join(supportedOps(), ", "))
	}
	return nil
}

func supportedOps() []string {
	ops := make([]string, 0, len(validOps))
	for op := range validOps {
		ops = append(ops, op)
	}
	return ops
}

// --- 比较逻辑 ---

func compare(bbVal any, op string, target any) bool {
	switch op {
	case "in":
		return compareIn(bbVal, target)
	default:
		return compareValues(bbVal, op, target)
	}
}

// compareValues 比较两个值，支持 float64/int64/string/bool
func compareValues(a any, op string, b any) bool {
	// 统一数值类型为 float64 进行比较
	af, aIsNum := toFloat64(a)
	bf, bIsNum := toFloat64(b)

	if aIsNum && bIsNum {
		return compareFloat64(af, op, bf)
	}

	// 字符串比较
	as, aIsStr := a.(string)
	bs, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return compareString(as, op, bs)
	}

	// bool 比较（只支持 == 和 !=）
	ab, aIsBool := a.(bool)
	bb, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		switch op {
		case "==":
			return ab == bb
		case "!=":
			return ab != bb
		default:
			return false
		}
	}

	// 类型不匹配
	return false
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

// compareIn 检查 bbVal 是否在 target 数组中
func compareIn(bbVal any, target any) bool {
	arr, ok := target.([]any)
	if !ok {
		return false
	}

	bvf, bvIsNum := toFloat64(bbVal)
	bvs, bvIsStr := bbVal.(string)

	for _, item := range arr {
		if bvIsNum {
			if f, isNum := toFloat64(item); isNum && f == bvf {
				return true
			}
		} else if bvIsStr {
			if s, isStr := item.(string); isStr && s == bvs {
				return true
			}
		}
	}
	return false
}

// --- 类型转换 ---

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// decodeJSONValue 将 json.RawMessage 解码为 Go 值
func decodeJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}

	// 尝试解析为各种类型
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}
