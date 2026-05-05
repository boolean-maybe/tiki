package plugin

import (
	"sort"
	"sync"
)

// renderableMetadataFields holds the set of field names the detail-view
// renderer can produce a primitive for. The view layer registers entries
// during package init (see view/taskdetail/field_registry.go); the workflow
// loader uses them to gate metadata: validation so users get a clear error
// for unsupported fields instead of silent placeholders at runtime.
//
// Mutated only at init time, but guarded with a mutex for tests that may
// register concurrently.
var (
	renderableMetadataMu     sync.RWMutex
	renderableMetadataFields = map[string]struct{}{}
)

// RegisterRenderableMetadataField marks name as a field the detail view can
// render. Safe to call from package init or from tests.
func RegisterRenderableMetadataField(name string) {
	if name == "" {
		return
	}
	renderableMetadataMu.Lock()
	renderableMetadataFields[name] = struct{}{}
	renderableMetadataMu.Unlock()
}

// isRenderableMetadataField reports whether name has been registered as
// renderable. Returns false for empty strings.
func isRenderableMetadataField(name string) bool {
	if name == "" {
		return false
	}
	renderableMetadataMu.RLock()
	defer renderableMetadataMu.RUnlock()
	_, ok := renderableMetadataFields[name]
	return ok
}

// renderableMetadataFieldList returns a sorted snapshot of the registered
// names. Used in error messages so users can see the supported set.
func renderableMetadataFieldList() []string {
	renderableMetadataMu.RLock()
	defer renderableMetadataMu.RUnlock()
	out := make([]string, 0, len(renderableMetadataFields))
	for name := range renderableMetadataFields {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
