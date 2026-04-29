package tikistore

import (
	"sort"
	"strings"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// GetAllTasks returns workflow-capable tasks sorted by priority then title.
//
// Presence-aware contract: plain documents (markdown files whose frontmatter
// lacks every workflow field — status/type/priority/points/tags/dependsOn/
// due/recurrence/assignee) are NOT returned here. Boards, burndown, lists,
// and any other surface that consumes GetAllTasks inherits this filter for
// free. To include plain docs (e.g. for search or markdown-view surfaces),
// callers should use a broader accessor once one exists.
func (s *TikiStore) GetAllTasks() []*taskpkg.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*taskpkg.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if !t.IsWorkflow {
			continue
		}
		tasks = append(tasks, t)
	}
	sortTasks(tasks)
	return tasks
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

// Search searches workflow tasks matching the given query.
//
// Presence-aware contract: a nil filterFunc means "all workflow tasks",
// matching GetAllTasks. Plain docs (IsWorkflow=false) are never returned by
// this API — Phase 1 Search is a task-search surface, not a general
// document-search surface; a later phase will add a separate
// document-search API that includes plain markdown.
//
// Callers that want to include plain docs must pass an explicit filterFunc
// (which bypasses the workflow gate, since a custom filter is assumed to
// know what it's selecting for).
//
// query: case-insensitive search term (searches task IDs, titles,
// descriptions, and tags).
// filterFunc: nil → workflow tasks only; non-nil → caller-defined filter.
// Returns matching tasks sorted by priority then title with relevance scores.
func (s *TikiStore) Search(query string, filterFunc func(*taskpkg.Task) bool) []taskpkg.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)

	// Step 1: Filter tasks using filterFunc (or the workflow gate if nil).
	var candidateTasks []*taskpkg.Task
	if filterFunc != nil {
		// Custom filter — assume caller knows what they're selecting for,
		// including plain docs if that's what they want.
		for _, t := range s.tasks {
			if filterFunc(t) {
				candidateTasks = append(candidateTasks, t)
			}
		}
	} else {
		// No filter = workflow tasks only, matching GetAllTasks semantics.
		for _, t := range s.tasks {
			if !t.IsWorkflow {
				continue
			}
			candidateTasks = append(candidateTasks, t)
		}
	}

	// Step 2: Apply search query if not empty
	var matchedTasks []*taskpkg.Task
	if queryLower == "" {
		// Empty query returns all candidate tasks
		matchedTasks = candidateTasks
	} else {
		// Filter by query
		for _, t := range candidateTasks {
			if matchesQuery(t, queryLower) {
				matchedTasks = append(matchedTasks, t)
			}
		}
	}

	// Step 3: Sort and convert to results
	sortTasks(matchedTasks)
	results := make([]taskpkg.SearchResult, len(matchedTasks))
	for i, t := range matchedTasks {
		results[i] = taskpkg.SearchResult{
			Task:  t,
			Score: 1.0, // Future: implement proper relevance scoring
		}
	}

	return results
}

// sortTasks sorts tasks by priority first (lower number = higher priority), then by title, then by ID as tiebreaker
func sortTasks(tasks []*taskpkg.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority < tasks[j].Priority
		}
		if tasks[i].Title != tasks[j].Title {
			return tasks[i].Title < tasks[j].Title
		}
		return tasks[i].ID < tasks[j].ID
	})
}
