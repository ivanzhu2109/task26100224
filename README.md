# scheduler — 教会月度志愿者排班库

一次输入覆盖一个自然月，每天只有上午、晚上两个互不重叠的场次。
`Generate` 在保留人工锁定指派、遵守团队资格与禁排窗口的前提下：

1. **第一目标**：最大化已填岗位总数（无法填满时返回最优的部分排班）；
2. **第二目标**：在覆盖数相同的方案中，最小化所有志愿者当月岗位数
   （含锁定指派）的平方和，即让月度工作量最均衡。

## 数据模型

- `Slot`：岗位，含唯一 `ID`、日期、场次（`Morning`/`Evening`）、所属团队。
- `Volunteer`：志愿者，可属多个团队，含禁排窗口 `[]Window`
  （按日期闭区间）与当月岗位上限 `Max`。
- `Lock`：岗位到志愿者的锁定指派，输入须合法且互不冲突；
  结果中原样保留，并占用对应场次与月度额度。
- `Plan`：一次排班输入（岗位、志愿者、锁定指派）。

## 使用

```go
p := scheduler.Plan{
    Slots: []scheduler.Slot{
        {ID: "s1", Date: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
            Period: scheduler.Morning, Team: "worship"},
    },
    Volunteers: []scheduler.Volunteer{
        {ID: "alice", Teams: []string{"worship"}, Max: 4,
            Blackout: []scheduler.Window{{Start: d1, End: d2}}},
    },
    Locks: []scheduler.Lock{{SlotID: "s1", VolunteerID: "alice"}},
}
assignments := scheduler.Generate(p) // []Assignment{SlotID, VolunteerID}
```

约束：志愿者必须属于岗位团队；同一人在同一场次最多一个岗位；
禁排日期内不可排班；当月岗位数不超过个人上限（锁定指派同样计入）。

## 实现

最小费用最大流：源点 → 岗位 → (志愿者, 场次) → 志愿者 → 汇点。
志愿者到汇点按凸费用拆边（第 j 份额外负载费用 2·(locked+j)−1），
使总费用恰为负载平方和；最大流保证覆盖数最优，同流量下最小费用
保证均衡性最优，两目标严格按优先级成立。

## 测试

```sh
go test ./...
```
