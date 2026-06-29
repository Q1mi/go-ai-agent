package plan

import (
	"fmt"
	"sort"
	"strings"
)

// Levels 对应课件 4.9 的 Kahn 拓扑分层算法。
// 同一层内的任务没有相互依赖，可以并行执行。
func Levels(plan Plan) ([][]string, error) {
	exists := make(map[string]bool, len(plan.Tasks))
	for _, task := range plan.Tasks {
		task.ID = strings.TrimSpace(task.ID)
		if task.ID == "" {
			return nil, fmt.Errorf("任务 ID 不能为空")
		}
		if exists[task.ID] {
			return nil, fmt.Errorf("任务 ID %q 重复", task.ID)
		}
		exists[task.ID] = true
	}

	indegree := make(map[string]int, len(plan.Tasks))
	dependents := make(map[string][]string)
	for _, task := range plan.Tasks {
		if _, ok := indegree[task.ID]; !ok {
			indegree[task.ID] = 0
		}
		for _, dep := range task.DependsOn {
			dep = strings.TrimSpace(dep)
			if !exists[dep] {
				return nil, fmt.Errorf("任务 %q 依赖了不存在的任务 %q", task.ID, dep)
			}
			indegree[task.ID]++
			dependents[dep] = append(dependents[dep], task.ID)
		}
	}

	var current []string
	for id, degree := range indegree {
		if degree == 0 {
			current = append(current, id)
		}
	}

	var levels [][]string
	done := 0
	for len(current) > 0 {
		sort.Strings(current)
		level := append([]string(nil), current...)
		levels = append(levels, level)
		done += len(level)

		var next []string
		for _, id := range level {
			for _, dependent := range dependents[id] {
				indegree[dependent]--
				if indegree[dependent] == 0 {
					next = append(next, dependent)
				}
			}
		}
		current = next
	}

	if done != len(indegree) {
		return nil, fmt.Errorf("计划存在循环依赖，无法执行")
	}
	return levels, nil
}
