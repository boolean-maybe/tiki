package task

import (
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

// normalizeStringSet trims values, drops empties, and removes duplicates.
func NormalizeStringSet(values []string) []string {
	return collectionutil.NormalizeStringSet(values)
}

// normalizeRefSet trims values, uppercases refs, drops empties, and removes duplicates.
func NormalizeRefSet(values []string) []string {
	return collectionutil.NormalizeRefSet(values)
}

// normalizeCollectionFields applies set-like normalization to collection fields.
func NormalizeCollectionFields(t *Task) {
	if t == nil {
		return
	}

	t.Tags = NormalizeStringSet(t.Tags)
	t.DependsOn = NormalizeRefSet(t.DependsOn)

	for name, raw := range t.CustomFields {
		fd, ok := workflow.Field(name)
		if !ok || !fd.Custom {
			continue
		}
		switch fd.Type {
		case workflow.TypeListString:
			ss, ok := stringSliceValue(raw)
			if !ok {
				continue
			}
			t.CustomFields[name] = NormalizeStringSet(ss)
		case workflow.TypeListRef:
			ss, ok := stringSliceValue(raw)
			if !ok {
				continue
			}
			t.CustomFields[name] = NormalizeRefSet(ss)
		}
	}
}

func stringSliceValue(raw interface{}) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		copyValue := make([]string, len(v))
		copy(copyValue, v)
		return copyValue, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}
