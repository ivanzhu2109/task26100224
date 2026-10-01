package scheduler

import "container/heap"

// edge 是最小费用流中的一条有向边。
type edge struct {
	to, rev   int // 终点及其反向边在邻接表中的下标
	cap, cost int // 剩余容量与单位费用
}

// minCostFlow 在 n 个节点的图上从 s 到 t 求最小费用最大流。
// 返回 (最大流量, 该流量下的最小费用)。初始费用必须非负。
func minCostFlow(n, s, t int, edges [][4]int) (int, int, [][]edge) {
	g := make([][]edge, n)
	add := func(u, v, cap, cost int) {
		g[u] = append(g[u], edge{to: v, rev: len(g[v]), cap: cap, cost: cost})
		g[v] = append(g[v], edge{to: u, rev: len(g[u]) - 1, cap: 0, cost: -cost})
	}
	for _, e := range edges {
		add(e[0], e[1], e[2], e[3])
	}

	const inf = int(^uint(0) >> 1)
	pot := make([]int, n) // Johnson 势，初始费用非负故全零可行
	flow, cost := 0, 0
	for {
		dist := make([]int, n)
		prevV := make([]int, n)
		prevE := make([]int, n)
		for i := range dist {
			dist[i] = inf
		}
		dist[s] = 0
		pq := &intHeap{{v: s, d: 0}}
		for pq.Len() > 0 {
			it := heap.Pop(pq).(pqItem)
			if it.d > dist[it.v] {
				continue
			}
			for i, e := range g[it.v] {
				if e.cap <= 0 {
					continue
				}
				nd := it.d + e.cost + pot[it.v] - pot[e.to]
				if nd < dist[e.to] {
					dist[e.to] = nd
					prevV[e.to] = it.v
					prevE[e.to] = i
					heap.Push(pq, pqItem{v: e.to, d: nd})
				}
			}
		}
		if dist[t] == inf {
			break
		}
		for v := 0; v < n; v++ {
			if dist[v] < inf {
				pot[v] += dist[v]
			}
		}
		// 沿最短路增广 1 个单位（所有瓶颈容量均为 1 时必然如此，
		// 这里仍取路径最小容量以保持通用）。
		d := inf
		for v := t; v != s; v = prevV[v] {
			if c := g[prevV[v]][prevE[v]].cap; c < d {
				d = c
			}
		}
		flow += d
		for v := t; v != s; v = prevV[v] {
			e := &g[prevV[v]][prevE[v]]
			e.cap -= d
			g[v][e.rev].cap += d
			cost += d * e.cost
		}
	}
	return flow, cost, g
}

type pqItem struct{ v, d int }
type intHeap []pqItem

func (h intHeap) Len() int            { return len(h) }
func (h intHeap) Less(i, j int) bool  { return h[i].d < h[j].d }
func (h intHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *intHeap) Push(x interface{}) { *h = append(*h, x.(pqItem)) }
func (h *intHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}
