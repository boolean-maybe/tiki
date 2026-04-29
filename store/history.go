package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/document"
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

	// Use batched git operations to get all file versions at once.
	// Phase 2: pass the document root directly so git's pathspec recurses
	// into subdirectories (the old `<dir>/*.md` glob would have missed any
	// nested layout like `.doc/tiki/<ID>.md` once the store root moves up
	// to `.doc/`).
	allVersions, err := h.gitOps.AllFileVersionsSince(h.taskDir, h.windowStart, true)
	if err != nil {
		return fmt.Errorf("getting file versions: %w", err)
	}

	// Process each file's version history
	for filePath, versions := range allVersions {
		if len(versions) == 0 {
			continue
		}

		// Parse status from each version. The id is taken from the first
		// version that exposes one in its frontmatter; the filename is a
		// fallback only — with Phase 2 paths are mutable and identity is
		// frontmatter, so keying transitions by filename after a rename
		// would split one task's history into two.
		type versionStatus struct {
			when   time.Time
			status task.Status
			hash   string
		}

		var statuses []versionStatus
		var frontmatterID string
		for _, version := range versions {
			ident, err := parseIdentityFromContent(version.Content)
			if err != nil {
				return fmt.Errorf("parsing status for %s at %s: %w", filePath, version.Hash, err)
			}
			if frontmatterID == "" && ident.id != "" {
				frontmatterID = ident.id
			}
			if !ident.hasExplicitStat {
				// No explicit `status` at this version — skip rather than
				// default to DefaultStatus. Matches the presence-aware rule:
				// a file with only `priority: high` is not yet workflow work.
				continue
			}
			statuses = append(statuses, versionStatus{
				when:   version.When,
				status: ident.status,
				hash:   version.Hash,
			})
		}

		taskID := frontmatterID
		if taskID == "" {
			taskID = deriveTaskID(filepath.Base(filePath))
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

// versionIdentity is the per-git-version view of a file that burndown needs:
// the document id (if the frontmatter carries one), the recorded workflow
// status, and a flag that says "this version counts for burndown". The flag
// is tighter than IsWorkflowFrontmatter: burndown only includes versions
// that expose an explicit `status` field, not merely any workflow-coloured
// key. A file with `priority: high` but no `status` is NOT counted, matching
// the presence-aware rule in the plan ("burndown must include only documents
// with explicit workflow status from the active workflow registry").
type versionIdentity struct {
	id              string
	status          task.Status
	hasExplicitStat bool
}

// parseIdentityFromContent parses a git-versioned file's frontmatter and
// returns the burndown-relevant identity. The returned id is the bare
// frontmatter id (uppercased) when present; callers fall back to the
// filename only when id is empty, which protects transition bookkeeping
// from filename-based keying after renames.
//
// Replaces the old parseStatusFromContent, whose `isWorkflow` flag was too
// permissive: it returned true whenever any workflow-coloured key was
// present and then defaulted missing `status` to DefaultStatus, which
// silently promoted partial documents into the burndown chart.
func parseIdentityFromContent(content string) (versionIdentity, error) {
	var out versionIdentity
	frontmatter, _, err := ParseFrontmatter(content)
	if err != nil {
		return out, err
	}
	if frontmatter == "" {
		return out, nil
	}
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return out, err
	}

	if raw, ok := document.FrontmatterID(fm); ok {
		out.id = document.NormalizeID(raw)
	}

	// The burndown gate: require an explicit, non-empty `status` string.
	rawStatus, has := fm["status"]
	if !has {
		return out, nil
	}
	s, isStr := rawStatus.(string)
	if !isStr || s == "" {
		return out, nil
	}
	out.status = task.MapStatus(s)
	out.hasExplicitStat = true
	return out, nil
}

func isActiveStatus(status task.Status) bool {
	return task.IsActiveStatus(status)
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
