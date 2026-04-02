package bt

// --- Sequence ---

// Sequence 依次执行子节点，遇到 Failure 或 Running 立即返回，全部 Success 返回 Success
type Sequence struct {
	Children []Node
}

func (s *Sequence) Tick(ctx *Context) Status {
	for _, child := range s.Children {
		status := child.Tick(ctx)
		if status != Success {
			return status
		}
	}
	return Success
}

// --- Selector ---

// Selector 依次执行子节点，遇到 Success 或 Running 立即返回，全部 Failure 返回 Failure
type Selector struct {
	Children []Node
}

func (s *Selector) Tick(ctx *Context) Status {
	for _, child := range s.Children {
		status := child.Tick(ctx)
		if status != Failure {
			return status
		}
	}
	return Failure
}

// --- Parallel ---

// ParallelPolicy 并行节点的成功/失败判定策略
type ParallelPolicy int

const (
	// RequireAll 所有子节点 Success 才 Success，任一 Failure 则 Failure
	RequireAll ParallelPolicy = iota
	// RequireOne 任一子节点 Success 就 Success，全部 Failure 才 Failure
	RequireOne
)

// Parallel 并行执行所有子节点
type Parallel struct {
	Children []Node
	Policy   ParallelPolicy
}

func (p *Parallel) Tick(ctx *Context) Status {
	successCount := 0
	failureCount := 0

	for _, child := range p.Children {
		status := child.Tick(ctx)
		switch status {
		case Success:
			successCount++
		case Failure:
			failureCount++
		case Running:
			// Running 不计入成功或失败
		}
	}

	switch p.Policy {
	case RequireAll:
		if failureCount > 0 {
			return Failure
		}
		if successCount == len(p.Children) {
			return Success
		}
		return Running
	case RequireOne:
		if successCount > 0 {
			return Success
		}
		if failureCount == len(p.Children) {
			return Failure
		}
		return Running
	default:
		return Failure
	}
}
