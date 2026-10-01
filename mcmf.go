package churchsched

import "container/heap"

// 最小费用最大流：第一目标最大化流量（已填岗位数），
// 在最大流中选取费用最小者（费用对应负载平方和的增量）。

type mcmfEdge struct {
	to, rev int
	cap     int
	cost    int
}

type mcmfGraph struct {
	n     int
	edges [][]mcmfEdge
}

func newMCMF(n int) *mcmfGraph {
	return &mcmfGraph{n: n, edges: make([][]mcmfEdge, n)}
}

// addEdge 添加一条容量为 cap、单位费用为 cost 的有向边及反向边。
func (g *mcmfGraph) addEdge(from, to, cap, cost int) {
	g.edges[from] = append(g.edges[from], mcmfEdge{to: to, rev: len(g.edges[to]), cap: cap, cost: cost})
	g.edges[to] = append(g.edges[to], mcmfEdge{to: from, rev: len(g.edges[from]) - 1, cap: 0, cost: -cost})
}

type spItem struct {
	node  int
	dist  int
	index int
}

type spQueue []spItem

func (q spQueue) Len() int            { return len(q) }
func (q spQueue) Less(i, j int) bool  { return q[i].dist < q[j].dist }
func (q spQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i]; q[i].index, q[j].index = i, j }
func (q *spQueue) Push(x interface{}) { *q = append(*q, x.(spItem)) }
func (q *spQueue) Pop() interface{} {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}

// minCostMaxFlow 从 s 到 t 求最小费用最大流，返回 (最大流, 最小费用)。
// 使用逐条最短路增广（SSP），势函数保证约化费用非负，可用 Dijkstra。
func (g *mcmfGraph) minCostMaxFlow(s, t int) (flow, cost int) {
	const inf = int(^uint(0) >> 1)
	pot := make([]int, g.n) // 初始费用均非负，势从零开始即可
	for {
		dist := make([]int, g.n)
		prevV := make([]int, g.n)
		prevE := make([]int, g.n)
		for i := range dist {
			dist[i] = inf
		}
		dist[s] = 0
		pq := &spQueue{{node: s, dist: 0}}
		heap.Init(pq)
		for pq.Len() > 0 {
			it := heap.Pop(pq).(spItem)
			if it.dist > dist[it.node] {
				continue
			}
			for ei, e := range g.edges[it.node] {
				if e.cap <= 0 {
					continue
				}
				nd := it.dist + e.cost + pot[it.node] - pot[e.to]
				if nd < dist[e.to] {
					dist[e.to] = nd
					prevV[e.to] = it.node
					prevE[e.to] = ei
					heap.Push(pq, spItem{node: e.to, dist: nd})
				}
			}
		}
		if dist[t] == inf {
			return flow, cost
		}
		for v := 0; v < g.n; v++ {
			if dist[v] < inf {
				pot[v] += dist[v]
			}
		}
		// 本图所有边容量均为 1 或按单位拆分，增广量取路径最小容量。
		d := inf
		for v := t; v != s; v = prevV[v] {
			if c := g.edges[prevV[v]][prevE[v]].cap; c < d {
				d = c
			}
		}
		for v := t; v != s; v = prevV[v] {
			e := &g.edges[prevV[v]][prevE[v]]
			e.cap -= d
			g.edges[v][e.rev].cap += d
			cost += d * e.cost
		}
		flow += d
	}
}
