package tikistore

import (
	"log/slog"
	"sort"
	"strings"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// GetAllTasks returns all tasks, sorted by priority then title
func (s *TikiStore) GetAllTasks() []*taskpkg.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*taskpkg.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	sortTasks(tasks)
	return tasks
}

// GetTasksByStatus returns tasks filtered by status, sorted by priority then title
func (s *TikiStore) GetTasksByStatus(status taskpkg.Status) []*taskpkg.Task {
	slog.Debug("retrieving tasks by status", "status", status)
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetColumn := taskpkg.StatusColumn(status)

	var tasks []*taskpkg.Task
	for _, t := range s.tasks {
		if taskpkg.StatusColumn(t.Status) == targetColumn {
			tasks = append(tasks, t)
		}
	}
	sortTasks(tasks)
	return tasks
}

// GetBacklogTasks returns tasks with backlog status, sorted by priority then title
func (s *TikiStore) GetBacklogTasks() []*taskpkg.Task {
	slog.Debug("retrieving backlog tasks")
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*taskpkg.Task
	for _, t := range s.tasks {
		if taskpkg.StatusColumn(t.Status) == taskpkg.StatusBacklog {
			tasks = append(tasks, t)
		}
	}
	sortTasks(tasks)
	return tasks
}

// SearchBacklog searches backlog tasks by title (case-insensitive).
// Returns results with Score for relevance (currently all 1.0, sorted by priority then title).
func (s *TikiStore) SearchBacklog(query string) []taskpkg.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	if query == "" {
		// Return all backlog tasks
		var tasks []*taskpkg.Task
		for _, t := range s.tasks {
			if taskpkg.StatusColumn(t.Status) == taskpkg.StatusBacklog {
				tasks = append(tasks, t)
			}
		}
		sortTasks(tasks)
		results := make([]taskpkg.SearchResult, len(tasks))
		for i, t := range tasks {
			results[i] = taskpkg.SearchResult{Task: t, Score: 1.0}
		}
		return results
	}

	queryLower := strings.ToLower(query)
	var tasks []*taskpkg.Task
	for _, t := range s.tasks {
		if taskpkg.StatusColumn(t.Status) == taskpkg.StatusBacklog {
			if strings.Contains(strings.ToLower(t.Title), queryLower) {
				tasks = append(tasks, t)
			}
		}
	}
	sortTasks(tasks)
	results := make([]taskpkg.SearchResult, len(tasks))
	for i, t := range tasks {
		results[i] = taskpkg.SearchResult{Task: t, Score: 1.0}
	}
	return results
}

// Search searches tasks with optional filter function.
// query: case-insensitive search term (searches task titles)
// filterFunc: filter function to pre-filter tasks (nil = all tasks)
// Returns matching tasks sorted by priority then title with relevance scores.
func (s *TikiStore) Search(query string, filterFunc func(*taskpkg.Task) bool) []taskpkg.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)

	// Step 1: Filter tasks using filterFunc (or include all if nil)
	var candidateTasks []*taskpkg.Task
	if filterFunc != nil {
		// Apply custom filter function
		for _, t := range s.tasks {
			if filterFunc(t) {
				candidateTasks = append(candidateTasks, t)
			}
		}
	} else {
		// No filter = all tasks
		for _, t := range s.tasks {
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
			if strings.Contains(strings.ToLower(t.Title), queryLower) {
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

// sortTasks sorts tasks by priority first (lower number = higher priority), then by title alphabetically
func sortTasks(tasks []*taskpkg.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		// First compare by priority (lower number = higher priority)
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority < tasks[j].Priority
		}

		// If priority is the same, sort by Title alphabetically
		return tasks[i].Title < tasks[j].Title
	})
}
