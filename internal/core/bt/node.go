package bt

// Status 行为树节点执行结果
type Status int

const (
	Success Status = iota
	Failure
	Running
)

func (s Status) String() string {
	switch s {
	case Success:
		return "Success"
	case Failure:
		return "Failure"
	case Running:
		return "Running"
	default:
		return "Unknown"
	}
}

// Node 行为树节点接口
type Node interface {
	Tick(ctx *Context) Status
}
