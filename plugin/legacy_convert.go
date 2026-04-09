package plugin

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/boolean-maybe/tiki/workflow"
)

// fieldNameMap maps lowercase field names to their canonical form.
// Built once from workflow.Fields() + the "tag"→"tags" alias.
var (
	fieldNameMap  map[string]string
	fieldNameOnce sync.Once
)

func buildFieldNameMap() {
	fieldNameOnce.Do(func() {
		fields := workflow.Fields()
		fieldNameMap = make(map[string]string, len(fields)+1)
		for _, f := range fields {
			fieldNameMap[strings.ToLower(f.Name)] = f.Name
		}
		fieldNameMap["tag"] = "tags" // singular alias
	})
}

// normalizeFieldName returns the canonical field name for a case-insensitive input.
// Returns the input unchanged if not found in the catalog.
func normalizeFieldName(name string) string {
	buildFieldNameMap()
	if canonical, ok := fieldNameMap[strings.ToLower(name)]; ok {
		return canonical
	}
	return name
}

// isArrayField returns true if the normalized field name is a list type (tags, dependsOn).
func isArrayField(name string) bool {
	canonical := normalizeFieldName(name)
	f, ok := workflow.Field(canonical)
	if !ok {
		return false
	}
	return f.Type == workflow.TypeListString || f.Type == workflow.TypeListRef
}

// LegacyConfigTransformer converts old-format workflow expressions to ruki.
type LegacyConfigTransformer struct{}

// NewLegacyConfigTransformer creates a new transformer.
func NewLegacyConfigTransformer() *LegacyConfigTransformer {
	return &LegacyConfigTransformer{}
}

// ConvertPluginConfig converts all legacy expressions in a plugin config in-place.
// Returns the number of fields converted.
func (t *LegacyConfigTransformer) ConvertPluginConfig(cfg *pluginFileConfig) int {
	count := 0

	// convert sort once (shared across lanes)
	var convertedSort string
	if cfg.Sort != "" {
		convertedSort = t.ConvertSort(cfg.Sort)
		slog.Warn("workflow.yaml uses deprecated 'sort' field — consider using 'order by' in lane filters",
			"plugin", cfg.Name)
	}

	// convert lane filters and actions
	for i := range cfg.Lanes {
		lane := &cfg.Lanes[i]

		if lane.Filter != "" && !isRukiFilter(lane.Filter) {
			newFilter := t.ConvertFilter(lane.Filter)
			slog.Debug("converted legacy filter", "old", lane.Filter, "new", newFilter, "lane", lane.Name)
			lane.Filter = newFilter
			count++
		}

		// merge sort into lane filter
		if convertedSort != "" {
			lane.Filter = mergeSortIntoFilter(lane.Filter, convertedSort)
		}

		if lane.Action != "" && !isRukiAction(lane.Action) {
			newAction, err := t.ConvertAction(lane.Action)
			if err != nil {
				slog.Warn("failed to convert legacy action, passing through",
					"error", err, "action", lane.Action, "lane", lane.Name)
				continue
			}
			slog.Debug("converted legacy action", "old", lane.Action, "new", newAction, "lane", lane.Name)
			lane.Action = newAction
			count++
		}
	}

	// convert plugin-level shortcut actions
	for i := range cfg.Actions {
		action := &cfg.Actions[i]
		if action.Action != "" && !isRukiAction(action.Action) {
			newAction, err := t.ConvertAction(action.Action)
			if err != nil {
				slog.Warn("failed to convert legacy plugin action, passing through",
					"error", err, "action", action.Action, "key", action.Key)
				continue
			}
			slog.Debug("converted legacy plugin action", "old", action.Action, "new", newAction, "key", action.Key)
			action.Action = newAction
			count++
		}
	}

	// clear sort after merging
	if cfg.Sort != "" {
		count++
		cfg.Sort = ""
	}

	return count
}

// isRukiFilter returns true if the expression is already in ruki format.
func isRukiFilter(expr string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(expr)), "select")
}

// isRukiAction returns true if the expression is already in ruki format.
func isRukiAction(expr string) bool {
	lower := strings.ToLower(strings.TrimSpace(expr))
	return strings.HasPrefix(lower, "update") ||
		strings.HasPrefix(lower, "create") ||
		strings.HasPrefix(lower, "delete")
}

// mergeSortIntoFilter appends an order-by clause to a filter, respecting existing order-by.
func mergeSortIntoFilter(filter, orderBy string) string {
	if strings.Contains(strings.ToLower(filter), "order by") {
		return filter
	}
	if filter == "" {
		return "select " + orderBy
	}
	return filter + " " + orderBy
}

// --- Filter conversion ---
//
// Transformation order (documented for maintenance):
//  1. Single quotes → double quotes
//  2. == → = (before any =-based pattern matching)
//  3. tag singular alias rewrites (before generic keyword lowering)
//  4. tags/array field IN/NOT IN expansion (before generic IN lowering)
//  5. NOT IN → not in (before standalone NOT)
//  6. Remaining keyword lowering: AND, OR, NOT, IN
//  7. NOW → now(), CURRENT_USER → user()
//  8. Field name normalization
//  9. Duration plural stripping

// pre-compiled regexes for filter conversion
var (
	// step 1: single quotes → double quotes
	reSingleQuoted = regexp.MustCompile(`'([^']*)'`)

	// step 2: == → =
	reDoubleEquals = regexp.MustCompile(`==`)

	// step 3: tag singular alias — equality: tag = "value" → "value" in tags
	reTagEquality = regexp.MustCompile(`(?i)\btag\b\s*=\s*("(?:[^"]*)")`)

	// step 3: tag singular alias — IN: tag IN [...] (captured together with tags IN below)
	// step 4: field IN [...] and field NOT IN [...]
	reFieldNotIn = regexp.MustCompile(`(?i)(\w+)\s+NOT\s+IN\s*\[([^\]]*)\]`)
	reFieldIn    = regexp.MustCompile(`(?i)(\w+)\s+IN\s*\[([^\]]*)\]`)

	// step 5: NOT IN → not in (word-bounded to avoid partial matches)
	reNotIn = regexp.MustCompile(`(?i)\bNOT\s+IN\b`)

	// step 6: keyword lowering
	reAnd = regexp.MustCompile(`(?i)\bAND\b`)
	reOr  = regexp.MustCompile(`(?i)\bOR\b`)
	reNot = regexp.MustCompile(`(?i)\bNOT\b`)
	reIn  = regexp.MustCompile(`(?i)\bIN\b`)

	// step 7: NOW → now(), CURRENT_USER → user()
	reNow         = regexp.MustCompile(`(?i)\bNOW\b`)
	reCurrentUser = regexp.MustCompile(`(?i)\bCURRENT_USER\b`)

	// step 8: field name normalization — matches identifiers before operators
	reFieldBeforeOp = regexp.MustCompile(`\b([a-zA-Z]\w*)\b`)

	// step 9: duration plural stripping — e.g. 24hours → 24hour, 1weeks → 1week
	reDurationUnit = regexp.MustCompile(`(\d+)(hour|minute|day|week|month|year)s\b`)
)

// ConvertFilter converts an old-format filter expression to a ruki select statement.
func (t *LegacyConfigTransformer) ConvertFilter(filter string) string {
	if filter == "" {
		return ""
	}

	s := strings.TrimSpace(filter)

	// step 1: single quotes → double quotes
	s = reSingleQuoted.ReplaceAllString(s, `"$1"`)

	// step 2: == → =
	s = reDoubleEquals.ReplaceAllString(s, "=")

	// step 3: tag singular alias — equality
	// tag = "value" → "value" in tags
	s = reTagEquality.ReplaceAllStringFunc(s, func(match string) string {
		m := reTagEquality.FindStringSubmatch(match)
		return m[1] + " in tags"
	})

	// step 3+4: field NOT IN [...] (must come before field IN)
	s = reFieldNotIn.ReplaceAllStringFunc(s, func(match string) string {
		m := reFieldNotIn.FindStringSubmatch(match)
		fieldName := m[1]
		values := m[2]
		return expandNotInClause(fieldName, values)
	})

	// step 4: field IN [...]
	s = reFieldIn.ReplaceAllStringFunc(s, func(match string) string {
		m := reFieldIn.FindStringSubmatch(match)
		fieldName := m[1]
		values := m[2]
		return expandInClause(fieldName, values)
	})

	// step 5: NOT IN → not in
	s = reNotIn.ReplaceAllString(s, "not in")

	// step 6: keyword lowering
	s = reAnd.ReplaceAllString(s, "and")
	s = reOr.ReplaceAllString(s, "or")
	s = reNot.ReplaceAllString(s, "not")
	s = reIn.ReplaceAllString(s, "in")

	// step 7: NOW → now(), CURRENT_USER → user()
	s = reNow.ReplaceAllString(s, "now()")
	s = reCurrentUser.ReplaceAllString(s, "user()")

	// step 8: field name normalization
	s = normalizeFieldNames(s)

	// step 9: duration plural stripping
	s = reDurationUnit.ReplaceAllString(s, "${1}${2}")

	return "select where " + s
}

// expandInClause handles field IN [...] expansion.
// For array fields (tags, dependsOn): expands to ("v1" in field or "v2" in field).
// For scalar fields: lowercases to field in [...].
func expandInClause(fieldName, valuesStr string) string {
	if isArrayField(fieldName) {
		canonical := normalizeFieldName(fieldName)
		values := parseBracketValues(valuesStr)
		if len(values) == 1 {
			return values[0] + " in " + canonical
		}
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = v + " in " + canonical
		}
		return "(" + strings.Join(parts, " or ") + ")"
	}
	// scalar field: just lowercase the IN
	canonical := normalizeFieldName(fieldName)
	return canonical + " in [" + normalizeQuotedValues(valuesStr) + "]"
}

// expandNotInClause handles field NOT IN [...] expansion.
// For array fields: expands to ("v1" not in field and "v2" not in field).
// For scalar fields: lowercases to field not in [...].
func expandNotInClause(fieldName, valuesStr string) string {
	if isArrayField(fieldName) {
		canonical := normalizeFieldName(fieldName)
		values := parseBracketValues(valuesStr)
		if len(values) == 1 {
			return values[0] + " not in " + canonical
		}
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = v + " not in " + canonical
		}
		return "(" + strings.Join(parts, " and ") + ")"
	}
	canonical := normalizeFieldName(fieldName)
	return canonical + " not in [" + normalizeQuotedValues(valuesStr) + "]"
}

// parseBracketValues parses a comma-separated list of values from inside [...].
// Ensures all values are double-quoted.
func parseBracketValues(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// strip existing quotes and re-quote with double quotes
		p = strings.Trim(p, `"'`)
		result = append(result, `"`+p+`"`)
	}
	return result
}

// normalizeQuotedValues ensures all values in a comma-separated list use double quotes.
func normalizeQuotedValues(s string) string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.Trim(p, `"'`)
		result = append(result, `"`+p+`"`)
	}
	return strings.Join(result, ", ")
}

// normalizeFieldNames replaces known field names with their canonical forms.
// Only replaces identifiers outside of double-quoted strings.
func normalizeFieldNames(s string) string {
	buildFieldNameMap()
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		// skip quoted strings verbatim
		if s[i] == '"' {
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			if j < len(s) {
				j++ // include closing quote
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}

		// try to match an identifier at current position
		loc := reFieldBeforeOp.FindStringIndex(s[i:])
		if loc == nil {
			b.WriteString(s[i:])
			break
		}

		// check if there's a quote in the gap before this match — if so,
		// only advance to that quote and let the next iteration skip it
		gap := s[i : i+loc[0]]
		if qIdx := strings.IndexByte(gap, '"'); qIdx >= 0 {
			b.WriteString(s[i : i+qIdx])
			i += qIdx
			continue
		}

		// write non-identifier text before the match
		b.WriteString(gap)
		match := s[i+loc[0] : i+loc[1]]

		lower := strings.ToLower(match)
		if canonical, ok := fieldNameMap[lower]; ok {
			b.WriteString(canonical)
		} else {
			b.WriteString(match)
		}
		i += loc[1]
	}

	return b.String()
}

// --- Sort conversion ---

// ConvertSort converts an old-format sort expression to a ruki order-by clause.
func (t *LegacyConfigTransformer) ConvertSort(sort string) string {
	if sort == "" {
		return ""
	}

	parts := strings.Split(sort, ",")
	converted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		tokens := strings.Fields(p)
		if len(tokens) == 0 {
			continue
		}
		fieldName := normalizeFieldName(tokens[0])
		if len(tokens) > 1 && strings.EqualFold(tokens[1], "DESC") {
			converted = append(converted, fieldName+" desc")
		} else {
			converted = append(converted, fieldName)
		}
	}
	return "order by " + strings.Join(converted, ", ")
}

// --- Action conversion ---

// ConvertAction converts an old-format action expression to a ruki update statement.
func (t *LegacyConfigTransformer) ConvertAction(action string) (string, error) {
	if action == "" {
		return "", nil
	}

	segments, err := splitTopLevelCommas(strings.TrimSpace(action))
	if err != nil {
		return "", fmt.Errorf("splitting action segments: %w", err)
	}

	setParts := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		part, err := convertActionSegment(seg)
		if err != nil {
			return "", fmt.Errorf("converting segment %q: %w", seg, err)
		}
		setParts = append(setParts, part)
	}

	return "update where id = id() set " + strings.Join(setParts, " "), nil
}

// convertActionSegment converts a single assignment like "status='ready'" or "tags+=[frontend]".
func convertActionSegment(seg string) (string, error) {
	// detect += and -= operators first
	if idx := strings.Index(seg, "+="); idx > 0 {
		fieldName := normalizeFieldName(strings.TrimSpace(seg[:idx]))
		value := strings.TrimSpace(seg[idx+2:])
		converted := convertBracketValues(value)
		return fieldName + "=" + fieldName + "+" + converted, nil
	}
	if idx := strings.Index(seg, "-="); idx > 0 {
		fieldName := normalizeFieldName(strings.TrimSpace(seg[:idx]))
		value := strings.TrimSpace(seg[idx+2:])
		converted := convertBracketValues(value)
		return fieldName + "=" + fieldName + "-" + converted, nil
	}

	// simple = assignment
	idx := strings.Index(seg, "=")
	if idx < 0 {
		return "", fmt.Errorf("no assignment operator found in %q", seg)
	}
	// handle == (old format uses both = and ==)
	fieldName := normalizeFieldName(strings.TrimSpace(seg[:idx]))
	value := strings.TrimSpace(seg[idx+1:])
	if strings.HasPrefix(value, "=") {
		value = strings.TrimSpace(value[1:]) // skip second =
	}

	// convert single quotes to double quotes
	value = reSingleQuoted.ReplaceAllString(value, `"$1"`)
	// convert CURRENT_USER
	value = reCurrentUser.ReplaceAllString(value, "user()")
	// quote bare identifiers
	value = quoteIfBareIdentifier(value)

	return fieldName + "=" + value, nil
}

// convertBracketValues converts a bracket-enclosed list, quoting bare identifiers.
// e.g. [frontend, 'needs review'] → ["frontend", "needs review"]
func convertBracketValues(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		// not bracket-enclosed, just quote it
		s = reSingleQuoted.ReplaceAllString(s, `"$1"`)
		return quoteIfBareIdentifier(s)
	}

	inner := s[1 : len(s)-1]
	parts := strings.Split(inner, ",")
	converted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// strip existing quotes
		p = strings.Trim(p, `"'`)
		// re-quote — all values in action brackets must be strings
		converted = append(converted, `"`+p+`"`)
	}
	return "[" + strings.Join(converted, ", ") + "]"
}

var (
	// matches function calls like now(), user(), id()
	reFunctionCall = regexp.MustCompile(`^\w+\(\)$`)
	// matches numeric values (int or float)
	reNumeric = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	// matches bare identifiers (not already quoted, not numeric, not a function)
	reBareIdentifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// quoteIfBareIdentifier wraps a value in double quotes if it's a bare identifier
// (not numeric, not a function call, not already quoted).
func quoteIfBareIdentifier(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, `"`) {
		return value // already quoted
	}
	if reNumeric.MatchString(value) {
		return value // numeric
	}
	if reFunctionCall.MatchString(value) {
		return value // function call
	}
	if reBareIdentifier.MatchString(value) {
		return `"` + value + `"`
	}
	return value
}

// splitTopLevelCommas splits a string on commas, respecting [...] brackets and quotes.
func splitTopLevelCommas(input string) ([]string, error) {
	var result []string
	var current strings.Builder
	depth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteByte(ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteByte(ch)
		case ch == '[' && !inSingle && !inDouble:
			depth++
			current.WriteByte(ch)
		case ch == ']' && !inSingle && !inDouble:
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unmatched ']' at position %d", i)
			}
			current.WriteByte(ch)
		case ch == ',' && depth == 0 && !inSingle && !inDouble:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}

	if depth != 0 {
		return nil, fmt.Errorf("unmatched '[' in expression")
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in expression")
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result, nil
}
