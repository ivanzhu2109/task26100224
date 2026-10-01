// Package scheduler 提供教会月度志愿者排班能力。
//
// 输入覆盖一个自然月，每天只有上午、晚上两个互不重叠的场次。
// Generate 在保留锁定指派、遵守团队资格与禁排窗口的前提下，
// 首先最大化已填岗位数，其次最小化所有志愿者当月岗位数的平方和。
package scheduler

import "time"

// Period 表示一天内的场次。
type Period int

const (
	Morning Period = iota // 上午场
	Evening               // 晚上场
)

// Slot 表示一个需要一名志愿者的岗位。
type Slot struct {
	ID     string    // 岗位唯一标识
	Date   time.Time // 所在日期（仅日期部分有意义）
	Period Period    // 所属场次
	Team   string    // 所属团队
}

// Window 表示按日期闭区间计算的禁排窗口。
type Window struct {
	Start time.Time // 起始日期（含）
	End   time.Time // 结束日期（含）
}

// Contains 报告日期 d 是否落在闭区间 [Start, End] 内（按天比较）。
func (w Window) Contains(d time.Time) bool {
	day := dayOnly(d)
	return !day.Before(dayOnly(w.Start)) && !day.After(dayOnly(w.End))
}

// Volunteer 表示一名志愿者，可以属于多个团队。
type Volunteer struct {
	ID       string   // 志愿者唯一标识
	Teams    []string // 所属团队列表
	Blackout []Window // 禁排窗口，窗口内任何日期都不能排班
	Max      int      // 当月最多承担的岗位数
}

// Lock 表示一条岗位到志愿者的锁定指派，生成结果中必须原样保留。
type Lock struct {
	SlotID      string
	VolunteerID string
}

// Plan 是一次排班输入，覆盖一个自然月。
type Plan struct {
	Slots      []Slot
	Volunteers []Volunteer
	Locks      []Lock
}

// Assignment 表示一个岗位被指派给一名志愿者。
type Assignment struct {
	SlotID      string
	VolunteerID string
}

// dayOnly 返回 t 所在日期的零点（UTC），用于按天比较。
func dayOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
