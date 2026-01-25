package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git"
	"github.com/boolean-maybe/tiki/task"

	"gopkg.in/yaml.v3"
)

const burndownDays = 14
const burndownHalfDays = burndownDays * 2 // 12-hour intervals (AM/PM)

type StatusChange struct {
	TaskID string
	From   task.Status
	To     task.Status
	At     time.Time
	Commit string
}

type BurndownPoint struct {
	Date      time.Time
	Remaining int
}

type TaskHistory struct {
	gitOps       git.GitOps
	taskDir      string
	now          func() time.Time
	windowStart  time.Time
	transitions  map[string][]StatusChange
	baseActive   int
	activeDeltas []statusDelta
}

type statusEvent struct {
	when   time.Time
	status task.Status
}

type statusDelta struct {
	when  time.Time
	delta int
}

func NewTaskHistory(taskDir string, gitOps git.GitOps) *TaskHistory {
	return &TaskHistory{
		gitOps:  gitOps,
		taskDir: taskDir,
		now:     time.Now,
	}
}

func (h *TaskHistory) Build() error {
	if h.gitOps == nil {
		return fmt.Errorf("git operations are required")
	}
	if h.taskDir == "" {
		return fmt.Errorf("task directory is required")
	}

	h.windowStart = dayStartUTC(h.now().UTC().AddDate(0, 0, -(burndownDays - 1)))
	h.transitions = make(map[string][]StatusChange)
	h.activeDeltas = nil
	h.baseActive = 0

	// Use batched git operations to get all file versions at once
	dirPattern := filepath.Join(h.taskDir, "*.md")
	allVersions, err := h.gitOps.AllFileVersionsSince(dirPattern, h.windowStart, true)
	if err != nil {
		return fmt.Errorf("getting file versions: %w", err)
	}

	// Process each file's version history
	for filePath, versions := range allVersions {
		if len(versions) == 0 {
			continue
		}

		taskID := deriveTaskID(filepath.Base(filePath))

		// Parse status from each version
		type versionStatus struct {
			when   time.Time
			status task.Status
			hash   string
		}

		var statuses []versionStatus
		for _, version := range versions {
			status, err := parseStatusFromContent(version.Content)
			if err != nil {
				return fmt.Errorf("parsing status for %s at %s: %w", filePath, version.Hash, err)
			}
			statuses = append(statuses, versionStatus{
				when:   version.When,
				status: status,
				hash:   version.Hash,
			})
		}

		// Build events from statuses (same logic as before)
		var baselineStatus task.Status
		baselineSet := false
		for _, s := range statuses {
			if s.when.Before(h.windowStart) {
				baselineStatus = s.status
				baselineSet = true
			}
		}

		var events []statusEvent
		var lastStatus task.Status
		hasStatus := false

		if baselineSet {
			events = append(events, statusEvent{
				when:   h.windowStart,
				status: baselineStatus,
			})
			lastStatus = baselineStatus
			hasStatus = true
		}

		for _, s := range statuses {
			if s.when.Before(h.windowStart) {
				continue
			}

			if !hasStatus {
				events = append(events, statusEvent{when: s.when, status: s.status})
				lastStatus = s.status
				hasStatus = true
				continue
			}

			if s.status == lastStatus {
				continue
			}

			h.transitions[taskID] = append(h.transitions[taskID], StatusChange{
				TaskID: taskID,
				From:   lastStatus,
				To:     s.status,
				At:     s.when,
				Commit: s.hash,
			})
			events = append(events, statusEvent{when: s.when, status: s.status})
			lastStatus = s.status
		}

		if len(events) > 0 {
			h.recordEvents(events)
		}
	}

	sort.SliceStable(h.activeDeltas, func(i, j int) bool {
		return h.activeDeltas[i].when.Before(h.activeDeltas[j].when)
	})

	return nil
}

func (h *TaskHistory) Transitions() map[string][]StatusChange {
	return h.transitions
}

func (h *TaskHistory) Burndown() []BurndownPoint {
	if h.windowStart.IsZero() {
		return nil
	}

	points := make([]BurndownPoint, 0, burndownHalfDays)
	current := h.baseActive
	eventIndex := 0

	periodStart := h.windowStart
	for i := 0; i < burndownHalfDays; i++ {
		periodEnd := periodStart.Add(12 * time.Hour)
		for eventIndex < len(h.activeDeltas) && !h.activeDeltas[eventIndex].when.After(periodEnd) {
			current += h.activeDeltas[eventIndex].delta
			eventIndex++
		}

		points = append(points, BurndownPoint{
			Date:      periodStart,
			Remaining: current,
		})
		periodStart = periodEnd
	}

	return points
}

func (h *TaskHistory) recordEvents(events []statusEvent) {
	if len(events) == 0 {
		return
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].when.Before(events[j].when)
	})

	lastStatus := events[0].status
	if events[0].when.Equal(h.windowStart) && isActiveStatus(lastStatus) {
		h.baseActive++
	} else if isActiveStatus(lastStatus) {
		h.activeDeltas = append(h.activeDeltas, statusDelta{when: events[0].when, delta: 1})
	}

	for i := 1; i < len(events); i++ {
		prevActive := isActiveStatus(lastStatus)
		nextActive := isActiveStatus(events[i].status)
		if prevActive == nextActive {
			lastStatus = events[i].status
			continue
		}

		delta := -1
		if nextActive {
			delta = 1
		}
		h.activeDeltas = append(h.activeDeltas, statusDelta{
			when:  events[i].when,
			delta: delta,
		})
		lastStatus = events[i].status
	}
}

func parseStatusFromContent(content string) (task.Status, error) {
	frontmatter, _, err := ParseFrontmatter(content)
	if err != nil {
		return task.StatusBacklog, err
	}

	if frontmatter == "" {
		return task.StatusBacklog, nil
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return task.StatusBacklog, err
	}

	statusVal := task.StatusBacklog
	if rawStatus, ok := fm["status"]; ok {
		if s, ok := rawStatus.(string); ok && s != "" {
			statusVal = task.MapStatus(s)
		}
	}

	return statusVal, nil
}

func isActiveStatus(status task.Status) bool {
	pane := task.StatusPane(status)
	return pane == task.StatusReady || pane == task.StatusInProgress || pane == task.StatusReview
}

func deriveTaskID(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if base == "" {
		return ""
	}
	return strings.ToUpper(base)
}

func dayStartUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
