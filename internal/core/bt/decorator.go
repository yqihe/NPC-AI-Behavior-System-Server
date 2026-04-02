package bt

// Inverter 反转子节点结果：Success↔Failure，Running 保持不变
type Inverter struct {
	Child Node
}

func (inv *Inverter) Tick(ctx *Context) Status {
	status := inv.Child.Tick(ctx)
	switch status {
	case Success:
		return Failure
	case Failure:
		return Success
	default:
		return status
	}
}
