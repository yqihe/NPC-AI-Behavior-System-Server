package bt

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// Context 行为树 Tick 时的上下文，传递给每个节点
type Context struct {
	BB *blackboard.Blackboard
}
