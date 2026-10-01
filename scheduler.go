package scheduler

import (
	"fmt"
	"sort"
)

// sessionKey 唯一标识一个场次（日期 + 上午/晚上）。
type sessionKey string

func slotSession(s Slot) sessionKey {
	return sessionKey(fmt.Sprintf("%s/%d", dayOnly(s.Date).Format("2006-01-02"), s.Period))
}

// vsKey 标识一个 (志愿者, 场次) 组合。
type vsKey struct {
	vi int
	sk sessionKey
}

// Generate 生成排班结果，语义见包文档。
//
// 建模为最小费用最大流：
//   - 源点 -> 每个未锁定岗位（容量 1）；
//   - 岗位 -> 每个有资格且当日不在禁排窗口内的 (志愿者, 场次) 节点（容量 1）；
//   - (志愿者, 场次) -> 志愿者（容量 1，若该场次已被锁定占用则容量 0）；
//   - 志愿者 -> 汇点：为第 j 份额外负载拆一条容量 1、费用 2*(locked+j)-1
//     的边，使总费用恰好等于含锁定指派在内的负载平方和。
//
// 最大流保证第一目标（已填岗位数最多），同流量下最小费用保证第二目标
// （平方和最小），两者严格按优先级成立。
func Generate(p Plan) []Assignment {
	lockedBySlot := make(map[string]string, len(p.Locks))
	for _, l := range p.Locks {
		lockedBySlot[l.SlotID] = l.VolunteerID
	}
	volIndex := make(map[string]int, len(p.Volunteers))
	for i, v := range p.Volunteers {
		volIndex[v.ID] = i
	}
	slotByID := make(map[string]Slot, len(p.Slots))
	for _, s := range p.Slots {
		slotByID[s.ID] = s
	}

	teamSet := make([]map[string]bool, len(p.Volunteers))
	for i, v := range p.Volunteers {
		teamSet[i] = make(map[string]bool, len(v.Teams))
		for _, t := range v.Teams {
			teamSet[i][t] = true
		}
	}
	eligible := func(vi int, s Slot) bool {
		if !teamSet[vi][s.Team] {
			return false
		}
		for _, w := range p.Volunteers[vi].Blackout {
			if w.Contains(s.Date) {
				return false
			}
		}
		return true
	}

	// 锁定指派占用的 (志愿者, 场次) 与月度额度。
	lockedSession := make(map[vsKey]bool)
	lockedCount := make([]int, len(p.Volunteers))
	for _, l := range p.Locks {
		vi := volIndex[l.VolunteerID]
		lockedCount[vi]++
		lockedSession[vsKey{vi, slotSession(slotByID[l.SlotID])}] = true
	}

	// 收集 (志愿者,场次) 节点：资格产生的 + 锁定占用的，统一编号。
	vsIndex := make(map[vsKey]int)
	var vsList []vsKey
	vsOf := func(k vsKey) int {
		if id, ok := vsIndex[k]; ok {
			return id
		}
		id := len(vsList)
		vsIndex[k] = id
		vsList = append(vsList, k)
		return id
	}

	var openSlots []int // 未锁定岗位在 p.Slots 中的下标
	for i, s := range p.Slots {
		if _, ok := lockedBySlot[s.ID]; ok {
			continue
		}
		openSlots = append(openSlots, i)
		for vi := range p.Volunteers {
			if eligible(vi, s) {
				vsOf(vsKey{vi, slotSession(s)})
			}
		}
	}
	for k := range lockedSession {
		vsOf(k)
	}

	// 节点编号：源点 0，岗位 [1, 1+ns)，(志愿者,场次) 随后，志愿者再后，汇点最后。
	ns := len(openSlots)
	vsBase := 1 + ns
	volBase := vsBase + len(vsList)
	sink := volBase + len(p.Volunteers)
	n := sink + 1

	var edges [][4]int
	add := func(u, v, cap, cost int) {
		edges = append(edges, [4]int{u, v, cap, cost})
	}
	// 源点 -> 岗位。
	for j := range openSlots {
		add(0, 1+j, 1, 0)
	}
	// 岗位 -> (志愿者, 场次)。
	for j, i := range openSlots {
		s := p.Slots[i]
		for vi := range p.Volunteers {
			if eligible(vi, s) {
				add(1+j, vsBase+vsOf(vsKey{vi, slotSession(s)}), 1, 0)
			}
		}
	}
	// (志愿者, 场次) -> 志愿者。
	for id, k := range vsList {
		cap := 1
		if lockedSession[k] {
			cap = 0
		}
		add(vsBase+id, volBase+k.vi, cap, 0)
	}
	// 志愿者 -> 汇点：凸费用拆边，第 j 条边费用 2*(locked+j)-1。
	for vi, v := range p.Volunteers {
		for j := 1; j <= v.Max-lockedCount[vi]; j++ {
			add(volBase+vi, sink, 1, 2*(lockedCount[vi]+j)-1)
		}
	}

	_, _, g := minCostFlow(n, 0, sink, edges)

	// 提取结果：锁定指派原样保留；岗位 -> (志愿者,场次) 的边容量耗尽即被占用。
	out := make([]Assignment, 0, len(p.Locks))
	for _, l := range p.Locks {
		out = append(out, Assignment{SlotID: l.SlotID, VolunteerID: l.VolunteerID})
	}
	for j, i := range openSlots {
		u := 1 + j
		for _, e := range g[u] {
			if e.to == 0 || e.cap != 0 {
				continue
			}
			vi := vsList[e.to-vsBase].vi
			out = append(out, Assignment{SlotID: p.Slots[i].ID, VolunteerID: p.Volunteers[vi].ID})
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].SlotID != out[b].SlotID {
			return out[a].SlotID < out[b].SlotID
		}
		return out[a].VolunteerID < out[b].VolunteerID
	})
	return out
}
