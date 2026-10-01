// Package churchsched 生成教会月度志愿者排班。
package churchsched

import "time"

// Session 表示一天内的场次，每天只有上午、晚上两个互不重叠的场次。
type Session int

const (
	Morning Session = iota // 上午场
	Evening                // 晚上场
)

// Team 表示一个服事团队。
type Team string

// Slot 表示一个待填岗位：属于某日某场次、某团队，需要一名志愿者。
type Slot struct {
	ID      string
	Date    time.Time // 只取日期部分
	Session Session
	Team    Team
}

// DateRange 为按日期闭区间计算的禁排窗口（Start、End 均含当日）。
type DateRange struct {
	Start time.Time
	End   time.Time
}

// Volunteer 表示一名志愿者。
type Volunteer struct {
	ID          string
	Teams       []Team      // 可属于多个团队
	Unavailable []DateRange // 禁排窗口，闭区间
	MaxMonthly  int         // 当月最多承担的岗位数（含锁定指派）
}

// Assignment 表示一条岗位到志愿者的指派。
type Assignment struct {
	SlotID      string
	VolunteerID string
}

// Plan 为一次排班输入，覆盖一个自然月。
type Plan struct {
	Slots      []Slot
	Volunteers []Volunteer
	Locked     []Assignment // 锁定指派，必须原样保留
}

// Schedule 为生成的排班结果，包含全部锁定指派。
type Schedule struct {
	Assignments []Assignment
}

// BySlot 返回岗位到志愿者的映射，便于查询。
func (s Schedule) BySlot() map[string]string {
	m := make(map[string]string, len(s.Assignments))
	for _, a := range s.Assignments {
		m[a.SlotID] = a.VolunteerID
	}
	return m
}

// Loads 返回每名志愿者当月承担的岗位数（含锁定指派）。
func (s Schedule) Loads() map[string]int {
	m := make(map[string]int)
	for _, a := range s.Assignments {
		m[a.VolunteerID]++
	}
	return m
}
