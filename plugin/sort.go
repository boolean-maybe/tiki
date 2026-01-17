package plugin

import (
	"sort"
	"strings"

	"github.com/boolean-maybe/tiki/task"
)

// SortRule represents a single sort criterion
type SortRule struct {
	Field      string // "Assignee", "Points", "Priority", "CreatedAt", "UpdatedAt", "Status", "Type", "Title"
	Descending bool   // true for DESC, false for ASC (default)
}

// ParseSort parses a sort expression like "Assignee, Points DESC, Priority DESC, CreatedAt"
func ParseSort(expr string) ([]SortRule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil // no custom sorting
	}

	var rules []SortRule
	parts := strings.Split(expr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}

		rule := SortRule{
			Field:      normalizeFieldName(fields[0]),
			Descending: false,
		}

		if len(fields) > 1 && strings.ToUpper(fields[1]) == "DESC" {
			rule.Descending = true
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// normalizeFieldName normalizes field names to a canonical form
func normalizeFieldName(field string) string {
	switch strings.ToLower(field) {
	case "assignee":
		return "assignee"
	case "points":
		return "points"
	case "priority":
		return "priority"
	case "createdat":
		return "createdat"
	case "updatedat":
		return "updatedat"
	case "status":
		return "status"
	case "type":
		return "type"
	case "title":
		return "title"
	case "id":
		return "id"
	default:
		return strings.ToLower(field)
	}
}

// SortTasks sorts tasks according to the given sort rules
func SortTasks(tasks []*task.Task, rules []SortRule) {
	if len(rules) == 0 {
		return // preserve original order
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		for _, rule := range rules {
			cmp := compareByField(tasks[i], tasks[j], rule.Field)
			if cmp != 0 {
				if rule.Descending {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false // equal by all criteria, preserve order
	})
}

// compareByField compares two tasks by a specific field
// Returns negative if a < b, 0 if equal, positive if a > b
func compareByField(a, b *task.Task, field string) int {
	switch field {
	case "assignee":
		return strings.Compare(strings.ToLower(a.Assignee), strings.ToLower(b.Assignee))
	case "points":
		return a.Points - b.Points
	case "priority":
		return a.Priority - b.Priority
	case "createdat":
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		} else if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return 0
	case "updatedat":
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return -1
		} else if a.UpdatedAt.After(b.UpdatedAt) {
			return 1
		}
		return 0
	case "status":
		return strings.Compare(string(a.Status), string(b.Status))
	case "type":
		return strings.Compare(string(a.Type), string(b.Type))
	case "title":
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	case "id":
		// Sort IDs lexicographically (alphanumeric IDs)
		return strings.Compare(strings.ToLower(a.ID), strings.ToLower(b.ID))
	default:
		return 0
	}
}
