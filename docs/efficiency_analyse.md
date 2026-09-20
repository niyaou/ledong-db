# Golang 效率统计中 `Analyse`（类似满班率）计算公式说明

## 重要前提

返回 JSON 中的 `analyse` 字段在产品中称为“满班率”，其实际业务含义是**“平均每节有效课程的成员数”**，计算逻辑和常见的“满班率 = 实际人数 / 满班容量”并不相同。单次班课使用独立上报人数参与该指标。

---

## 一、数据流总览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         数据库原始数据                                   │
│  course (课程)  +  spend (消费记录)  +  coach (教练)  +  court (场地)    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     getCourseStats()  SQL 聚合                           │
│  按 course.id 分组，统计每节课的：                                        │
│    • 普通课程 quantities_sum = SUM(spend.quantities)                     │
│    • 单次班课人数 = course.participant_count                             │
│    • spend_amount   = SUM( COALESCE(NULLIF(spend.charge,0),spend.description) )│
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Go 代码逐条计算 members / courses                     │
│  遍历每一节课的聚合结果，根据 quantities_sum 和 course_type 计算：        │
│    • courses = 1  (普通课有消费，或单次班课上报人数为正数)               │
│    • members 见下方详细公式                                              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     按教练/校区汇总并计算 Analyse                        │
│    Analyse = Members / Courses                                          │
│    （即平均每节课的"成员数"）                                            │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 二、核心计算公式

### 2.1 单节课的 Members（成员数）计算

```go
if quantities_sum > 0 {
    courses = 1
    if quantities_sum > 1 {
        members = quantities_sum          // 班课：直接取消费人数
    } else {
        members = course_type * 1         // 私教固定算 2，班课算 1
    }
}

if course_type == 3 && participant_count > 0 {
    courses = 1
    members = participant_count           // 单次班课：上报人数一比一计入
}
```

用表格表示：

| course_type | 含义 | quantities_sum | courses | members | 说明 |
|:-----------:|:----:|:--------------:|:-------:|:-------:|:-----|
| 1 | 班课 | 1 | 1 | 1 | 只有1人消费，按课程类型算 |
| 1 | 班课 | 5 | 1 | 5 | 多人消费，直接取实际人数 |
| 2 | 私教 | 1 | 1 | 2 | 私教固定算 2 人 |
| 2 | 私教 | 2 | 1 | 2 | 注意：代码里 quantities>1 走第一条分支，members=2 |
| 3 | 单次班课 | 上报 6 人 | 1 | 6 | 不依赖消费记录，上报人数一比一计入 |

> **奇怪点**：私教课即使 quantities_sum=1，members 也会被算成 **2** 而不是 1。这是因为代码里用了 `course_type * quantities`，而私教的 course_type = 2。

### 2.2 最终的 Analyse 计算

```go
// 按教练/校区汇总后
Analyse = Members / Courses
```

**举例**：
- 某教练有 10 节班课，每节课 3 人消费 → Members=30, Courses=10 → **Analyse = 3.0**
- 某教练有 10 节私教，每节课 1 人消费 → Members=20, Courses=10 → **Analyse = 2.0**
- 某教练有 2 节单次班课，分别上报 1 人和 6 人 → Members=7, Courses=2 → **Analyse = 3.5**

---

## 三、与"满班率"的区别

| 对比项 | Go 代码 `Analyse` | 通常理解的"满班率" |
|:------|:------------------|:------------------|
| **公式** | `Members / Courses` | `实际人数 / 满班容量` |
| **私教处理** | 固定按 2 人计算 | 按实际 1 人计算 |
| **班课处理** | 普通班课按实际消费人数，单次班课按上报人数 | 按实际到场人数 / 班级上限 |
| **是否有容量上限** | ❌ 没有 | ✅ 需要满班容量参数 |
| **取值范围** | 无上限，通常 1~5 | 0%~100% |

---

## 四、Python 脚本里之前的"满班率"

之前 Python 脚本中写的满班率：

```python
满班率 = 班课人次 / 班课数量
```

这个公式和 Go 代码的 `Analyse` **不完全一样**：
- Python 版只统计**班课**，Go 代码的 `Analyse` 混合了**班课+私教**
- Python 版普通班课按 `spend` 记录数、单次班课按 `participant_count` 计算；Go 代码的普通班课按 `SUM(quantities)` 计算
- Python 版私教是单独统计的，Go 代码私教被算成了 2 人

---

## 五、代码位置

| 文件 | 行号 | 函数/逻辑 |
|:-----|:-----|:----------|
| `internal/service/efficiency.go` | 98~133 | `getCourseStats()` SQL 聚合 |
| `internal/service/efficiency.go` | 192~202 | Members / Courses 逐条计算 |
| `internal/service/efficiency.go` | 319~346 | 按教练汇总并计算 `Analyse` |
| `internal/service/efficiency.go` | 328~336 | `item.Analyse = item.Members / item.Courses` |
