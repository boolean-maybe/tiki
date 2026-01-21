package plugin

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/task"
)

// PaneAction represents parsed pane actions.
type PaneAction struct {
	Ops []PaneActionOp
}

// PaneActionOp represents a single action operation.
type PaneActionOp struct {
	Field    ActionField
	Operator ActionOperator
	StrValue string
	IntValue int
	Tags     []string
}

// ActionField identifies a supported action field.
type ActionField string

const (
	ActionFieldStatus   ActionField = "status"
	ActionFieldType     ActionField = "type"
	ActionFieldPriority ActionField = "priority"
	ActionFieldAssignee ActionField = "assignee"
	ActionFieldPoints   ActionField = "points"
	ActionFieldTags     ActionField = "tags"
)

// ActionOperator identifies a supported action operator.
type ActionOperator string

const (
	ActionOperatorAssign ActionOperator = "="
	ActionOperatorAdd    ActionOperator = "+="
	ActionOperatorRemove ActionOperator = "-="
)

// ParsePaneAction parses a pane action string into operations.
func ParsePaneAction(input string) (PaneAction, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return PaneAction{}, nil
	}

	parts, err := splitTopLevelCommas(input)
	if err != nil {
		return PaneAction{}, err
	}

	ops := make([]PaneActionOp, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return PaneAction{}, fmt.Errorf("empty action segment")
		}

		field, op, value, err := parseActionSegment(part)
		if err != nil {
			return PaneAction{}, err
		}

		switch field {
		case ActionFieldTags:
			if op == ActionOperatorAssign {
				return PaneAction{}, fmt.Errorf("tags action only supports += or -=")
			}
			tags, err := parseTagsValue(value)
			if err != nil {
				return PaneAction{}, err
			}
			ops = append(ops, PaneActionOp{
				Field:    field,
				Operator: op,
				Tags:     tags,
			})
		case ActionFieldPriority, ActionFieldPoints:
			if op != ActionOperatorAssign {
				return PaneAction{}, fmt.Errorf("%s action only supports =", field)
			}
			intValue, err := parseIntValue(value)
			if err != nil {
				return PaneAction{}, err
			}
			if field == ActionFieldPriority && !task.IsValidPriority(intValue) {
				return PaneAction{}, fmt.Errorf("priority value out of range: %d", intValue)
			}
			if field == ActionFieldPoints && !task.IsValidPoints(intValue) {
				return PaneAction{}, fmt.Errorf("points value out of range: %d", intValue)
			}
			ops = append(ops, PaneActionOp{
				Field:    field,
				Operator: op,
				IntValue: intValue,
			})
		case ActionFieldStatus:
			if op != ActionOperatorAssign {
				return PaneAction{}, fmt.Errorf("%s action only supports =", field)
			}
			strValue, err := parseStringValue(value)
			if err != nil {
				return PaneAction{}, err
			}
			if _, ok := task.ParseStatus(strValue); !ok {
				return PaneAction{}, fmt.Errorf("invalid status value %q", strValue)
			}
			ops = append(ops, PaneActionOp{
				Field:    field,
				Operator: op,
				StrValue: strValue,
			})
		case ActionFieldType:
			if op != ActionOperatorAssign {
				return PaneAction{}, fmt.Errorf("%s action only supports =", field)
			}
			strValue, err := parseStringValue(value)
			if err != nil {
				return PaneAction{}, err
			}
			if _, ok := task.ParseType(strValue); !ok {
				return PaneAction{}, fmt.Errorf("invalid type value %q", strValue)
			}
			ops = append(ops, PaneActionOp{
				Field:    field,
				Operator: op,
				StrValue: strValue,
			})
		default:
			if op != ActionOperatorAssign {
				return PaneAction{}, fmt.Errorf("%s action only supports =", field)
			}
			strValue, err := parseStringValue(value)
			if err != nil {
				return PaneAction{}, err
			}
			ops = append(ops, PaneActionOp{
				Field:    field,
				Operator: op,
				StrValue: strValue,
			})
		}
	}

	return PaneAction{Ops: ops}, nil
}

// ApplyPaneAction applies a parsed action to a task clone.
func ApplyPaneAction(src *task.Task, action PaneAction, currentUser string) (*task.Task, error) {
	if src == nil {
		return nil, fmt.Errorf("task is nil")
	}

	if len(action.Ops) == 0 {
		return src.Clone(), nil
	}

	clone := src.Clone()
	for _, op := range action.Ops {
		switch op.Field {
		case ActionFieldStatus:
			clone.Status = task.MapStatus(op.StrValue)
		case ActionFieldType:
			clone.Type = task.NormalizeType(op.StrValue)
		case ActionFieldPriority:
			clone.Priority = op.IntValue
		case ActionFieldAssignee:
			assignee := op.StrValue
			if isCurrentUserToken(assignee) {
				if strings.TrimSpace(currentUser) == "" {
					return nil, fmt.Errorf("current user is not available for assignee")
				}
				assignee = currentUser
			}
			clone.Assignee = assignee
		case ActionFieldPoints:
			clone.Points = op.IntValue
		case ActionFieldTags:
			clone.Tags = applyTagOperation(clone.Tags, op.Operator, op.Tags)
		default:
			return nil, fmt.Errorf("unsupported action field %q", op.Field)
		}
	}

	if validation := task.QuickValidate(clone); validation.HasErrors() {
		return nil, fmt.Errorf("action resulted in invalid task: %w", validation)
	}

	return clone, nil
}

func isCurrentUserToken(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "CURRENT_USER")
}

func parseActionSegment(segment string) (ActionField, ActionOperator, string, error) {
	opIdx, op := findOperator(segment)
	if opIdx == -1 {
		return "", "", "", fmt.Errorf("action segment missing operator: %q", segment)
	}

	field := strings.TrimSpace(segment[:opIdx])
	value := strings.TrimSpace(segment[opIdx+len(op):])
	if field == "" || value == "" {
		return "", "", "", fmt.Errorf("invalid action segment: %q", segment)
	}

	switch strings.ToLower(field) {
	case "status":
		return ActionFieldStatus, op, value, nil
	case "type":
		return ActionFieldType, op, value, nil
	case "priority":
		return ActionFieldPriority, op, value, nil
	case "assignee":
		return ActionFieldAssignee, op, value, nil
	case "points":
		return ActionFieldPoints, op, value, nil
	case "tags":
		return ActionFieldTags, op, value, nil
	default:
		return "", "", "", fmt.Errorf("unknown action field %q", field)
	}
}

func findOperator(segment string) (int, ActionOperator) {
	if idx := strings.Index(segment, "+="); idx != -1 {
		return idx, ActionOperatorAdd
	}
	if idx := strings.Index(segment, "-="); idx != -1 {
		return idx, ActionOperatorRemove
	}
	if idx := strings.Index(segment, "="); idx != -1 {
		return idx, ActionOperatorAssign
	}
	return -1, ""
}

func parseStringValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') ||
			(value[0] == '"' && value[len(value)-1] == '"') {
			value = value[1 : len(value)-1]
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("string value is empty")
	}
	return value, nil
}

func parseIntValue(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value %q", value)
	}
	return intValue, nil
}

func parseTagsValue(raw string) ([]string, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("tags value must be in brackets, got %q", value)
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return nil, fmt.Errorf("tags list is empty")
	}
	parts, err := splitTopLevelCommas(inner)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag, err := parseStringValue(part)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func splitTopLevelCommas(input string) ([]string, error) {
	var parts []string
	start := 0
	inSingle := false
	inDouble := false
	bracketDepth := 0

	for i, r := range input {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '[':
			if !inSingle && !inDouble {
				bracketDepth++
			}
		case ']':
			if !inSingle && !inDouble {
				if bracketDepth == 0 {
					return nil, fmt.Errorf("unexpected ']' in %q", input)
				}
				bracketDepth--
			}
		case ',':
			if !inSingle && !inDouble && bracketDepth == 0 {
				part := strings.TrimSpace(input[start:i])
				parts = append(parts, part)
				start = i + 1
			}
		}
	}

	if inSingle || inDouble || bracketDepth != 0 {
		return nil, fmt.Errorf("unterminated quotes or brackets in %q", input)
	}

	part := strings.TrimSpace(input[start:])
	parts = append(parts, part)

	return parts, nil
}

func applyTagOperation(current []string, op ActionOperator, tags []string) []string {
	switch op {
	case ActionOperatorAdd:
		return addTags(current, tags)
	case ActionOperatorRemove:
		return removeTags(current, tags)
	default:
		return current
	}
}

func addTags(current []string, tags []string) []string {
	existing := make(map[string]bool, len(current))
	for _, tag := range current {
		existing[tag] = true
	}
	for _, tag := range tags {
		if !existing[tag] {
			current = append(current, tag)
			existing[tag] = true
		}
	}
	return current
}

func removeTags(current []string, tags []string) []string {
	toRemove := make(map[string]bool, len(tags))
	for _, tag := range tags {
		toRemove[tag] = true
	}
	filtered := current[:0]
	for _, tag := range current {
		if !toRemove[tag] {
			filtered = append(filtered, tag)
		}
	}
	return filtered
}
