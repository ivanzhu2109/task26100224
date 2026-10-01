# churchsched

教会月度志愿者排班库。在保留人工锁定指派、遵守团队资格与禁排窗口的前提下：

1. 第一目标：全局最大化已填岗位总数；
2. 第二目标：在覆盖数相同的方案中，最小化所有志愿者当月岗位数（含锁定）的平方和，使月度工作量最均衡。

## 模型

- 一次输入覆盖一个自然月，每天只有上午（`Morning`）、晚上（`Evening`）两个互不重叠的场次。
- 每个岗位（`Slot`）有唯一 ID，属于某日某场次、某团队（`Team`），需要一名志愿者；同一团队同一场次可有多个岗位。
- 志愿者（`Volunteer`）可属于多个团队，提供按日期闭区间计算的禁排窗口（`Unavailable`）与当月岗位上限（`MaxMonthly`，含锁定指派）。
- 锁定指派（`Plan.Locked`）原样保留，并同样占用场次与月度额度。
- 约束：志愿者必须属于岗位团队；同一人在同一（日期, 场次）最多一个岗位；禁排日内不可排；当月总数不超过个人上限。

## 实现

建模为最小费用最大流（`scheduler.go` + `mcmf.go`）：

```
源 -> 岗位(cap 1) -> (志愿者,日期,场次)(cap 1) -> 志愿者负载 -> 汇
```

负载到汇的第 j 条单位边费用为 `2*(locked+j)-1`，使总费用恰为
`sum(load^2) - sum(locked^2)`：最大流保证已填岗位数最多，
同流量下最小费用即负载平方和最小。无法填满时返回最优部分排班。

## 使用

```go
plan := churchsched.Plan{
    Slots: []churchsched.Slot{
        {ID: "p1", Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
            Session: churchsched.Morning, Team: "worship"},
        // ...
    },
    Volunteers: []churchsched.Volunteer{
        {ID: "alice", Teams: []churchsched.Team{"worship", "usher"},
            Unavailable: []churchsched.DateRange{{Start: d1, End: d2}},
            MaxMonthly: 4},
        // ...
    },
    Locked: []churchsched.Assignment{{SlotID: "p1", VolunteerID: "alice"}},
}
sched := churchsched.Generate(plan)
bySlot := sched.BySlot() // 岗位 ID -> 志愿者 ID（未填岗位不在其中）
loads := sched.Loads()   // 志愿者 ID -> 当月岗位数（含锁定）
```

## 测试

```sh
go test ./...
```
