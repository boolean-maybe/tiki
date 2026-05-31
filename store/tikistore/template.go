package tikistore

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/boolean-maybe/tiki/config"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

func (s *TikiStore) setAuthorFromIdentityTiki(tk *tikipkg.Tiki) {
	name, email, err := s.GetCurrentUser()
	if err != nil || (name == "" && email == "") {
		return
	}
	switch {
	case name != "" && email != "":
		tk.Set("createdBy", fmt.Sprintf("%s <%s>", name, email))
	case name != "":
		tk.Set("createdBy", name)
	default:
		tk.Set("createdBy", email)
	}
}

// NewTikiTemplate returns a new tiki populated with creation defaults from
// the loaded workflow field catalog. Every workflow field declaring a
// default contributes one Fields entry; enum fields contribute the value
// marked default: true, non-enum fields contribute their declared
// DefaultValue. Workflows that declare no defaults produce a bare tiki with
// only id/createdAt set — callers fill in title/body and any field values.
func (s *TikiStore) NewTikiTemplate() (*tikipkg.Tiki, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.newTikiTemplateLocked()
}

// newTikiTemplateLocked builds a template tiki. Caller must hold s.mu.
func (s *TikiStore) newTikiTemplateLocked() (*tikipkg.Tiki, error) {
	if err := config.RequireWorkflowFieldsLoaded(); err != nil {
		return nil, err
	}

	// Identity uniqueness is checked against the in-memory index (s.tikis),
	// not the filesystem — a tiki loaded from a renamed file occupies an id
	// without occupying <tikidir>/<id>.md, so an os.Stat probe would falsely
	// report the id free.
	var tikiID string
	for i := 0; ; i++ {
		candidate := normalizeTikiID(config.GenerateRandomID())
		if _, taken := s.tikis[candidate]; !taken {
			tikiID = candidate
			break
		}
		if i > maxGenerateRetries {
			return nil, fmt.Errorf("failed to generate unique tiki id after %d attempts", maxGenerateRetries)
		}
		slog.Debug("ID collision detected in index, regenerating", "id", candidate)
	}

	tk := tikipkg.New()
	tk.SetID(tikiID)
	tk.SetCreatedAt(time.Now())

	for k, v := range buildCustomFieldDefaults() {
		tk.Set(k, v)
	}

	s.setAuthorFromIdentityTiki(tk)

	return tk, nil
}

// buildCustomFieldDefaults collects creation defaults from all loaded
// workflow field definitions. For enum fields, the default is the value
// marked default: true. For non-enum fields, it's FieldDef.DefaultValue.
// Returns nil when no workflow field declares a default.
func buildCustomFieldDefaults() map[string]interface{} {
	var defaults map[string]interface{}
	add := func(name string, value interface{}) {
		if defaults == nil {
			defaults = make(map[string]interface{})
		}
		defaults[name] = value
	}
	for _, fd := range workflow.WorkflowFields() {
		if fd.Type == workflow.TypeEnum {
			if def := fd.EnumDefault(); def != "" {
				add(fd.Name, def)
			}
			continue
		}
		if fd.DefaultValue == nil {
			continue
		}
		if ss, ok := fd.DefaultValue.([]string); ok {
			switch fd.Type {
			case workflow.TypeListRef:
				add(fd.Name, collectionutil.NormalizeRefSet(ss))
			case workflow.TypeListString:
				add(fd.Name, collectionutil.NormalizeStringSet(ss))
			default:
				cp := make([]string, len(ss))
				copy(cp, ss)
				add(fd.Name, cp)
			}
		} else {
			add(fd.Name, fd.DefaultValue)
		}
	}
	return defaults
}
