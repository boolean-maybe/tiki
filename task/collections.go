package task

import "sort"

// Sort sorts tasks by title, then by ID as a tiebreaker. Sorting is stable
// within a tie. Priority-aware ordering moved to ruki (`order by priority`)
// once priority became a workflow enum — runtime callers that want
// urgency-first ordering should use the executor instead of this helper.
func Sort(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Title != tasks[j].Title {
			return tasks[i].Title < tasks[j].Title
		}
		return tasks[i].ID < tasks[j].ID
	})
}
