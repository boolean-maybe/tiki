package tikistore

import (
	"sort"
	"strings"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// hasAnyWorkflowField reports whether tk carries at least one workflow-
// declared field in its Fields map. Used by the persistence layer to gate
// behaviors that only apply when typed workflow data is present — this is
// presence-of-fields, not a workflow/plain classification.
func hasAnyWorkflowField(tk *tikipkg.Tiki) bool {
	if tk == nil {
		return false
	}
	for _, fd := range workflow.WorkflowFields() {
		if tk.Has(fd.Name) {
			return true
		}
	}
	return false
}

func matchesTikiQuery(tk *tikipkg.Tiki, queryLower string) bool {
	if tk == nil || queryLower == "" {
		return false
	}
	if strings.Contains(strings.ToLower(tk.ID()), queryLower) ||
		strings.Contains(strings.ToLower(tk.Title()), queryLower) ||
		strings.Contains(strings.ToLower(tk.Body()), queryLower) {
		return true
	}
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
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
		ti, tj := strings.ToLower(results[i].Title()), strings.ToLower(results[j].Title())
		if ti != tj {
			return ti < tj
		}
		return results[i].ID() < results[j].ID()
	})
	return results
}
