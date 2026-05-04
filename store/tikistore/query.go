package tikistore

import (
	"sort"
	"strings"

	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// GetAllTasks returns all tikis projected to tasks, sorted by priority then title.
// Plain docs are included; callers that want only workflow-capable items should
// filter with hasAnyWorkflowField or use a ruki query with has(status).
func (s *TikiStore) GetAllTasks() []*taskpkg.Task {
	tikis := s.GetAllTikis()
	tasks := make([]*taskpkg.Task, 0, len(tikis))
	for _, tk := range tikis {
		tasks = append(tasks, tikipkg.ToTask(tk))
	}
	taskpkg.Sort(tasks)
	return tasks
}

// hasAnyWorkflowField reports whether tk has at least one schema-known
// workflow field in its Fields map.
func hasAnyWorkflowField(tk *tikipkg.Tiki) bool {
	if tk == nil {
		return false
	}
	for _, f := range tikipkg.SchemaKnownFields {
		if tk.Has(f) {
			return true
		}
	}
	return false
}

func matchesQuery(task *taskpkg.Task, queryLower string) bool {
	if task == nil || queryLower == "" {
		return false
	}
	if strings.Contains(strings.ToLower(task.ID), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(task.Title), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(task.Description), queryLower) {
		return true
	}
	for _, tag := range task.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
}

func matchesTikiQuery(tk *tikipkg.Tiki, queryLower string) bool {
	if tk == nil || queryLower == "" {
		return false
	}
	return strings.Contains(strings.ToLower(tk.ID), queryLower) ||
		strings.Contains(strings.ToLower(tk.Title), queryLower) ||
		strings.Contains(strings.ToLower(tk.Body), queryLower)
}

// Search searches workflow tasks matching the given query.
//
// Presence-aware contract: a nil filterFunc means "all workflow tasks".
// Plain docs (no workflow fields) are never returned by this API.
// Callers that want to include plain docs should use SearchTikis.
func (s *TikiStore) Search(query string, filterFunc func(*taskpkg.Task) bool) []taskpkg.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)

	var candidateTasks []*taskpkg.Task
	if filterFunc != nil {
		for _, tk := range s.tikis {
			t := tikipkg.ToTask(tk)
			if filterFunc(t) {
				candidateTasks = append(candidateTasks, t)
			}
		}
	} else {
		for _, tk := range s.tikis {
			if !hasAnyWorkflowField(tk) {
				continue
			}
			candidateTasks = append(candidateTasks, tikipkg.ToTask(tk))
		}
	}

	var matchedTasks []*taskpkg.Task
	if queryLower == "" {
		matchedTasks = candidateTasks
	} else {
		for _, t := range candidateTasks {
			if matchesQuery(t, queryLower) {
				matchedTasks = append(matchedTasks, t)
			}
		}
	}

	taskpkg.Sort(matchedTasks)
	results := make([]taskpkg.SearchResult, len(matchedTasks))
	for i, t := range matchedTasks {
		results[i] = taskpkg.SearchResult{Task: t, Score: 1.0}
	}
	return results
}

// SearchTikis searches all tikis (including plain docs) with an optional
// tiki-native filter. query matches against id, title, and body.
// filter is applied before the text match; nil means no pre-filter.
// Results are sorted by title then id.
func (s *TikiStore) SearchTikis(query string, filter func(*tikipkg.Tiki) bool) []*tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []*tikipkg.Tiki
	for _, tk := range s.tikis {
		if filter != nil && !filter(tk) {
			continue
		}
		if queryLower != "" && !matchesTikiQuery(tk, queryLower) {
			continue
		}
		results = append(results, tk)
	}
	sort.Slice(results, func(i, j int) bool {
		ti, tj := strings.ToLower(results[i].Title), strings.ToLower(results[j].Title)
		if ti != tj {
			return ti < tj
		}
		return results[i].ID < results[j].ID
	})
	return results
}
