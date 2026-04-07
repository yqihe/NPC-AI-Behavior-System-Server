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

// --- NPC 实例 ---

var KeyNPCType = NewKey[string]("npc_type")    // NPC 类型名，创建时写入
var KeyNPCPosX = NewKey[float64]("npc_pos_x")  // NPC 位置 X，Runtime 每 Tick 更新
var KeyNPCPosZ = NewKey[float64]("npc_pos_z")  // NPC 位置 Z，Runtime 每 Tick 更新

// --- 行为追踪 ---

var KeyCurrentAction   = NewKey[string]("current_action")    // BT 当前执行的子行为名
var KeyAlertStartTick  = NewKey[int64]("alert_start_tick")   // 进入 Alarmed 状态的时间戳
var KeyExitCleanupDone = NewKey[string]("exit_cleanup_done") // FSM OnExit 清理完成标记

// --- 需求系统 ---

var KeyNeedLowest    = NewKey[string]("need_lowest")      // 当前最低需求名
var KeyNeedLowestVal = NewKey[float64]("need_lowest_val") // 当前最低需求值

// --- 情绪系统 ---

var KeyEmotionDominant    = NewKey[string]("emotion_dominant")      // 主导情绪名
var KeyEmotionDominantVal = NewKey[float64]("emotion_dominant_val") // 主导情绪值

// --- 记忆系统 ---

var KeyMemoryCount = NewKey[int64]("memory_count") // 当前记忆条目数

// --- 社交系统 ---

var KeyGroupID    = NewKey[string]("group_id")     // 群组 ID
var KeySocialRole = NewKey[string]("social_role")   // 社交角色（leader/follower）

// --- 移动系统 ---

var KeyMoveState = NewKey[string]("move_state") // 移动状态（idle/moving/arrived）
