package blackboard

// 所有 Blackboard Key 的唯一定义处。
// 新增 Key 必须加在这个文件中，禁止散落到其他文件。

// --- 威胁相关 ---

var KeyThreatLevel = NewKey[float64]("threat_level")       // 当前威胁等级 0~100，决策中心写入，FSM 读取
var KeyThreatSource = NewKey[string]("threat_source")       // 威胁来源 ID，决策中心写入
var KeyThreatExpireAt = NewKey[int64]("threat_expire_at")   // 威胁过期时间戳（毫秒），决策中心写入，FSM 读取

// --- 事件相关 ---

var KeyLastEventType = NewKey[string]("last_event_type") // 最近一次感知到的事件类型，决策中心写入，FSM 读取
var KeyCurrentTime = NewKey[int64]("current_time")       // 当前时间戳（毫秒），Runtime 每 Tick 更新

// --- FSM 状态 ---

var KeyFSMState = NewKey[string]("fsm_state") // 当前 FSM 状态名，FSM 引擎写入
