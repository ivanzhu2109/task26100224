package churchsched

import (
	"testing"
	"time"
)

func d(day int) time.Time { return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC) }

// 跨团队资格形成四个岗位的让位链：
// p1(A):{v1,v2} p2(B):{v2,v3} p3(C):{v3,v4} p4(D):{v4}
// 局部先到先得（p1->v2, p2->v3, p3->v4）会让 p4 空岗，
// 但存在全覆盖排班 p1->v1, p2->v2, p3->v3, p4->v4。
func TestCrossTeamChainFullCoverage(t *testing.T) {
	plan := Plan{
		Slots: []Slot{
			{ID: "p1", Date: d(1), Session: Morning, Team: "A"},
			{ID: "p2", Date: d(1), Session: Morning, Team: "B"},
			{ID: "p3", Date: d(1), Session: Morning, Team: "C"},
			{ID: "p4", Date: d(1), Session: Morning, Team: "D"},
		},
		Volunteers: []Volunteer{
			{ID: "v1", Teams: []Team{"A"}, MaxMonthly: 10},
			{ID: "v2", Teams: []Team{"A", "B"}, MaxMonthly: 10},
			{ID: "v3", Teams: []Team{"B", "C"}, MaxMonthly: 10},
			{ID: "v4", Teams: []Team{"C", "D"}, MaxMonthly: 10},
		},
	}
	got := Generate(plan).BySlot()
	for _, id := range []string{"p1", "p2", "p3", "p4"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("slot %s unfilled, want full coverage, got %v", id, got)
		}
	}
	if got["p1"] != "v1" || got["p2"] != "v2" || got["p3"] != "v3" || got["p4"] != "v4" {
		t.Fatalf("unexpected assignment: %v", got)
	}
}

// 锁定指派同时消耗场次与月度额度，叠加禁排窗口边界；
// 并在覆盖数相同、负载平方和不同时选择全局更均衡者。
func TestLockedQuotaBanAndBalance(t *testing.T) {
	t.Run("锁定消耗场次与月度额度且禁排边界闭区间", func(t *testing.T) {
		plan := Plan{
			Slots: []Slot{
				{ID: "pL", Date: d(1), Session: Morning, Team: "A"},
				{ID: "p0", Date: d(1), Session: Morning, Team: "A"}, // v1 场次被锁定占用 -> 空
				{ID: "p1", Date: d(2), Session: Morning, Team: "A"},
				{ID: "p2", Date: d(2), Session: Evening, Team: "A"},
				{ID: "pB0", Date: d(1), Session: Morning, Team: "B"}, // v2 禁排日 -> 空
				{ID: "pB1", Date: d(2), Session: Morning, Team: "B"}, // 禁排结束次日 -> 可排
			},
			Volunteers: []Volunteer{
				{ID: "v1", Teams: []Team{"A"}, MaxMonthly: 2},
				{ID: "v2", Teams: []Team{"B"}, MaxMonthly: 3,
					Unavailable: []DateRange{{Start: d(1), End: d(1)}}},
			},
			Locked: []Assignment{{SlotID: "pL", VolunteerID: "v1"}},
		}
		s := Generate(plan)
		got := s.BySlot()
		if got["pL"] != "v1" {
			t.Fatalf("locked assignment not preserved: %v", got)
		}
		if _, ok := got["p0"]; ok {
			t.Fatalf("p0 should stay empty: v1 session consumed by lock, got %v", got)
		}
		if _, ok := got["pB0"]; ok {
			t.Fatalf("pB0 should stay empty: v2 banned on day 1 (inclusive), got %v", got)
		}
		if got["pB1"] != "v2" {
			t.Fatalf("pB1 should be filled by v2 after ban window ends, got %v", got)
		}
		// v1 月度上限 2，锁定占 1，只剩 1 个额度：p1/p2 只能填一个。
		filled := 0
		for _, id := range []string{"p1", "p2"} {
			if got[id] == "v1" {
				filled++
			}
		}
		if filled != 1 {
			t.Fatalf("lock must consume monthly quota: exactly one of p1/p2 filled by v1, got %v", got)
		}
		if loads := s.Loads(); loads["v1"] != 2 || loads["v2"] != 1 {
			t.Fatalf("unexpected loads: %v", loads)
		}
	})

	t.Run("覆盖数相同选择负载平方和更小者", func(t *testing.T) {
		plan := Plan{
			Slots: []Slot{
				{ID: "pL", Date: d(1), Session: Morning, Team: "A"},
				{ID: "p1", Date: d(2), Session: Morning, Team: "A"},
				{ID: "p2", Date: d(2), Session: Evening, Team: "A"},
			},
			Volunteers: []Volunteer{
				{ID: "v1", Teams: []Team{"A"}, MaxMonthly: 5},
				{ID: "v2", Teams: []Team{"A"}, MaxMonthly: 5},
			},
			Locked: []Assignment{{SlotID: "pL", VolunteerID: "v1"}},
		}
		s := Generate(plan)
		got := s.BySlot()
		if got["pL"] != "v1" || got["p1"] == "" || got["p2"] == "" {
			t.Fatalf("want full coverage, got %v", got)
		}
		// v1 全拿 -> 负载 (3,0) 平方和 9；分摊 -> (2,1) 平方和 5。
		loads := s.Loads()
		sq := loads["v1"]*loads["v1"] + loads["v2"]*loads["v2"]
		if sq != 5 {
			t.Fatalf("want balanced loads (2,1) with square sum 5, got loads=%v sq=%d", loads, sq)
		}
	})
}
