package scheduler

import (
	"fmt"
	"testing"
	"time"
)

func d(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }

// validate 检查排班结果满足全部硬约束且保留锁定指派。
func validate(t *testing.T, p Plan, got []Assignment) {
	t.Helper()
	slots := make(map[string]Slot, len(p.Slots))
	for _, s := range p.Slots {
		slots[s.ID] = s
	}
	vols := make(map[string]Volunteer, len(p.Volunteers))
	for _, v := range p.Volunteers {
		vols[v.ID] = v
	}
	// 锁定指派原样保留。
	want := make(map[Assignment]bool, len(p.Locks))
	for _, l := range p.Locks {
		want[Assignment{SlotID: l.SlotID, VolunteerID: l.VolunteerID}] = true
	}
	for _, l := range p.Locks {
		found := false
		for _, a := range got {
			if a.SlotID == l.SlotID && a.VolunteerID == l.VolunteerID {
				found = true
			}
		}
		if !found {
			t.Errorf("锁定指派 %+v 未保留", l)
		}
	}
	_ = want
	filled := make(map[string]bool)
	inSession := make(map[string]map[string]bool) // 志愿者 -> 场次 -> 已占用
	load := make(map[string]int)
	for _, a := range got {
		s, ok := slots[a.SlotID]
		if !ok {
			t.Fatalf("指派了不存在的岗位 %q", a.SlotID)
		}
		v, ok := vols[a.VolunteerID]
		if !ok {
			t.Fatalf("指派了不存在的志愿者 %q", a.VolunteerID)
		}
		if filled[a.SlotID] {
			t.Errorf("岗位 %q 被重复填充", a.SlotID)
		}
		filled[a.SlotID] = true
		// 团队资格。
		inTeam := false
		for _, tm := range v.Teams {
			if tm == s.Team {
				inTeam = true
			}
		}
		if !inTeam {
			t.Errorf("志愿者 %q 不属于岗位 %q 的团队 %q", v.ID, s.ID, s.Team)
		}
		// 禁排窗口。
		for _, w := range v.Blackout {
			if w.Contains(s.Date) {
				t.Errorf("志愿者 %q 在禁排窗口内被排到岗位 %q", v.ID, s.ID)
			}
		}
		// 同一场次最多一个岗位。
		sk := fmt.Sprintf("%s/%d", dayOnly(s.Date), s.Period)
		if inSession[v.ID] == nil {
			inSession[v.ID] = map[string]bool{}
		}
		if inSession[v.ID][sk] {
			t.Errorf("志愿者 %q 在场次 %s 承担了多个岗位", v.ID, sk)
		}
		inSession[v.ID][sk] = true
		load[v.ID]++
	}
	// 月度上限。
	for id, n := range load {
		if n > vols[id].Max {
			t.Errorf("志愿者 %q 当月承担 %d 个岗位，超过上限 %d", id, n, vols[id].Max)
		}
	}
}

// 跨团队资格形成四个岗位的让位链：先到先得会留下空岗，但存在全覆盖排班。
//
// 志愿者：A 属于 T1+T2（上限 2），B 仅 T1（上限 1），C 仅 T2（上限 1）。
// 岗位（各占一个独立场次）：s1=T2, s2=T2, s3=T1, s4=T1。
// 按岗位顺序先到先得：s1->A, s2->A（A 满），s3->B，s4 无人可排。
// 最优解需要 A 让出 T2：s1->C, s2->A, s3->A, s4->B，实现全覆盖。
func TestYieldChainFullCoverage(t *testing.T) {
	p := Plan{
		Slots: []Slot{
			{ID: "s1", Date: d(1), Period: Morning, Team: "T2"},
			{ID: "s2", Date: d(2), Period: Morning, Team: "T2"},
			{ID: "s3", Date: d(3), Period: Morning, Team: "T1"},
			{ID: "s4", Date: d(4), Period: Morning, Team: "T1"},
		},
		Volunteers: []Volunteer{
			{ID: "A", Teams: []string{"T1", "T2"}, Max: 2},
			{ID: "B", Teams: []string{"T1"}, Max: 1},
			{ID: "C", Teams: []string{"T2"}, Max: 1},
		},
	}
	got := Generate(p)
	validate(t, p, got)
	if len(got) != 4 {
		t.Fatalf("期望全覆盖 4 个岗位，实际填充 %d 个：%v", len(got), got)
	}
}

// 锁定指派同时消耗场次与月度额度，叠加禁排窗口边界；
// 覆盖数相同时选择负载平方和更小的全局均衡解。
//
// 布局（均为团队 T）：
//   - L：10-05 上午，锁定给 X（占用 X 的该场次与 1 点月度额度）；
//   - s1：10-05 上午，与 L 同场次，X 被占用 -> 只能给 Y；
//   - s2：10-10 晚上，Y 的禁排窗口 [10-10, 10-10] 边界当天 -> 只能给 X；
//   - s3：10-12 晚上，窗口结束日之后，X、Y 都可排。
//
// X 上限 3、Y 上限 2。全覆盖有两种：s3->X 得负载 (3,1) 平方和 10，
// s3->Y 得负载 (2,2) 平方和 8，必须选择后者。
func TestLockBlackoutAndBalance(t *testing.T) {
	p := Plan{
		Slots: []Slot{
			{ID: "L", Date: d(5), Period: Morning, Team: "T"},
			{ID: "s1", Date: d(5), Period: Morning, Team: "T"},
			{ID: "s2", Date: d(10), Period: Evening, Team: "T"},
			{ID: "s3", Date: d(12), Period: Evening, Team: "T"},
		},
		Volunteers: []Volunteer{
			{ID: "X", Teams: []string{"T"}, Max: 3},
			{ID: "Y", Teams: []string{"T"}, Max: 2,
				Blackout: []Window{{Start: d(10), End: d(10)}}},
		},
		Locks: []Lock{{SlotID: "L", VolunteerID: "X"}},
	}
	got := Generate(p)
	validate(t, p, got)
	if len(got) != 4 {
		t.Fatalf("期望全覆盖 4 个岗位，实际填充 %d 个：%v", len(got), got)
	}
	asg := make(map[string]string, len(got))
	for _, a := range got {
		asg[a.SlotID] = a.VolunteerID
	}
	if asg["L"] != "X" {
		t.Errorf("锁定指派未保留：L -> %q", asg["L"])
	}
	if asg["s1"] != "Y" {
		t.Errorf("同场次锁定应迫使 s1 给 Y，实际给 %q", asg["s1"])
	}
	if asg["s2"] != "X" {
		t.Errorf("禁排窗口边界日应迫使 s2 给 X，实际给 %q", asg["s2"])
	}
	if asg["s3"] != "Y" {
		t.Errorf("覆盖数相同时应选负载 (2,2) 而非 (3,1)，s3 实际给 %q", asg["s3"])
	}
}

// 锁定指派消耗月度额度：X 上限 1 且已被锁定占满，
// 同团队另一场次岗位 s5 无人能排，应返回仅含锁定指派的最优部分排班。
func TestLockConsumesMonthlyQuota(t *testing.T) {
	p := Plan{
		Slots: []Slot{
			{ID: "L", Date: d(5), Period: Morning, Team: "T"},
			{ID: "s5", Date: d(6), Period: Morning, Team: "T"},
		},
		Volunteers: []Volunteer{
			{ID: "X", Teams: []string{"T"}, Max: 1},
			{ID: "Y", Teams: []string{"U"}, Max: 4}, // 不属于团队 T
		},
		Locks: []Lock{{SlotID: "L", VolunteerID: "X"}},
	}
	got := Generate(p)
	validate(t, p, got)
	if len(got) != 1 || got[0] != (Assignment{SlotID: "L", VolunteerID: "X"}) {
		t.Fatalf("期望仅保留锁定指派，实际：%v", got)
	}
}

// 无法填满时返回最优的部分排班：无资格志愿者的岗位留空，其余填满。
func TestPartialCoverage(t *testing.T) {
	p := Plan{
		Slots: []Slot{
			{ID: "s1", Date: d(1), Period: Morning, Team: "T"},
			{ID: "s2", Date: d(2), Period: Morning, Team: "ghost"}, // 无人有资格
		},
		Volunteers: []Volunteer{
			{ID: "A", Teams: []string{"T"}, Max: 5},
		},
	}
	got := Generate(p)
	validate(t, p, got)
	if len(got) != 1 || got[0] != (Assignment{SlotID: "s1", VolunteerID: "A"}) {
		t.Fatalf("期望仅填充 s1，实际：%v", got)
	}
}
