package churchsched

import "time"

// Generate 根据输入计划生成排班。
//
// 所有锁定指派原样保留；在遵守团队资格、同场次唯一、禁排窗口、
// 个人月度上限的前提下，先最大化已填岗位总数，
// 再最小化所有志愿者当月岗位数（含锁定）的平方和。
// 无法填满时返回最优的部分排班。
//
// 建模为最小费用最大流：
//   源 -> 岗位(1) -> (志愿者,日期,场次)(1) -> 志愿者负载 -> 汇。
// 负载到汇的第 j 条单位边费用为 2*(locked+j)-1，
// 使总费用恰为 sum(load^2) - sum(locked^2)，
// 最大流保证已填岗位数最大，同流量下最小费用即负载平方和最小。
func Generate(p Plan) Schedule {
	slots := make([]Slot, 0, len(p.Slots))
	slotByID := make(map[string]Slot, len(p.Slots))
	for _, s := range p.Slots {
		s.Date = dayOf(s.Date)
		slots = append(slots, s)
		slotByID[s.ID] = s
	}

	vols := make([]Volunteer, len(p.Volunteers))
	teamSet := make([]map[Team]bool, len(p.Volunteers))
	volIndex := make(map[string]int, len(p.Volunteers))
	for i, v := range p.Volunteers {
		vols[i] = v
		teamSet[i] = make(map[Team]bool, len(v.Teams))
		for _, t := range v.Teams {
			teamSet[i][t] = true
		}
		volIndex[v.ID] = i
	}

	// 处理锁定指派：原样保留，并占用场次与月度额度。
	result := make([]Assignment, 0, len(p.Slots))
	lockedCnt := make([]int, len(vols))
	type daySession struct {
		date time.Time
		sess Session
	}
	lockedSlot := make(map[string]bool, len(p.Locked))
	lockedSession := make([]map[daySession]bool, len(vols))
	for i := range lockedSession {
		lockedSession[i] = make(map[daySession]bool)
	}
	for _, a := range p.Locked {
		s := slotByID[a.SlotID]
		vi := volIndex[a.VolunteerID]
		result = append(result, a)
		lockedSlot[a.SlotID] = true
		lockedCnt[vi]++
		lockedSession[vi][daySession{s.Date, s.Session}] = true
	}

	// 待排岗位。
	open := make([]Slot, 0, len(slots)-len(p.Locked))
	for _, s := range slots {
		if !lockedSlot[s.ID] {
			open = append(open, s)
		}
	}

	// 节点编号：0=源, 1..n=岗位, 之后为 (志愿者,日期,场次) 节点,
	// 再之后为志愿者负载节点, 最后一个为汇。
	// 由于节点总数依赖场次节点个数，先收集候选边再建图。
	nSlots := len(open)
	nVols := len(vols)
	type dsKey struct {
		vol  int
		date time.Time
		sess Session
	}
	dsNodes := map[dsKey]int{}
	type slotEdge struct {
		slot int
		key  dsKey
	}
	var cand []slotEdge
	banned := func(vi int, d time.Time) bool {
		for _, w := range vols[vi].Unavailable {
			if !d.Before(dayOf(w.Start)) && !d.After(dayOf(w.End)) {
				return true
			}
		}
		return false
	}
	for si, s := range open {
		for vi := range vols {
			if !teamSet[vi][s.Team] || banned(vi, s.Date) {
				continue
			}
			if lockedCnt[vi] >= vols[vi].MaxMonthly {
				continue
			}
			key := dsKey{vi, s.Date, s.Session}
			if lockedSession[vi][daySession{s.Date, s.Session}] {
				continue
			}
			if _, ok := dsNodes[key]; !ok {
				dsNodes[key] = len(dsNodes)
			}
			cand = append(cand, slotEdge{si, key})
		}
	}

	nDS := len(dsNodes)
	base := 1 + nSlots
	dsBase := base
	loadBase := dsBase + nDS
	sinkNode := loadBase + nVols
	g := newMCMF(sinkNode + 1)

	for si := range open {
		g.addEdge(0, 1+si, 1, 0)
	}
	for _, ce := range cand {
		g.addEdge(1+ce.slot, dsBase+dsNodes[ce.key], 1, 0)
	}
	for key, idx := range dsNodes {
		g.addEdge(dsBase+idx, loadBase+key.vol, 1, 0)
	}
	for vi := range vols {
		remaining := vols[vi].MaxMonthly - lockedCnt[vi]
		for j := 1; j <= remaining; j++ {
			// 第 j 个新增岗位的边际费用：(locked+j)^2 - (locked+j-1)^2
			g.addEdge(loadBase+vi, sinkNode, 1, 2*(lockedCnt[vi]+j)-1)
		}
	}

	g.minCostMaxFlow(0, sinkNode)

	// 读取结果：岗位 -> 场次节点 的边被使用（容量耗尽）即表示已指派。
	volOfDS := make(map[int]int, len(dsNodes))
	for key, idx := range dsNodes {
		volOfDS[dsBase+idx] = key.vol
	}
	for si, s := range open {
		for _, e := range g.edges[1+si] {
			if e.cap == 0 && e.to != 0 {
				if vi, ok := volOfDS[e.to]; ok {
					result = append(result, Assignment{SlotID: s.ID, VolunteerID: vols[vi].ID})
					break
				}
			}
		}
	}
	return Schedule{Assignments: result}
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
