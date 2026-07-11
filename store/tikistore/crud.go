package tikistore

import (
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/boolean-maybe/ruki/idfmt"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// GetTiki retrieves a tiki by ID. Returns nil when not found.
func (s *TikiStore) GetTiki(id string) *tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tikis[normalizeTikiID(id)]
}

// CreateTiki adds a new tiki and saves it to a file.
func (s *TikiStore) CreateTiki(tk *tikipkg.Tiki) error {
	if err := s.createTikiLocked(tk); err != nil {
		return err
	}
	slog.Info("tiki created", "tiki_id", tk.ID())
	s.notifyListeners()
	return nil
}

func (s *TikiStore) createTikiLocked(tk *tikipkg.Tiki) error {
	return s.storeNewDocumentLocked(tk)
}

// storeNewDocumentLocked is the shared create path. The caller owns
// the workflow-presence decision — this function never adds workflow fields.
func (s *TikiStore) storeNewDocumentLocked(tk *tikipkg.Tiki) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// reject before assigning an ID or saving: a title that slugs to nothing
	// has no title-derived filename, and burning an ID on a doomed create
	// leaves partial state. both callers (TUI commit, ruki create) get one
	// uniform rejection point here.
	if document.Slugify(tk.Title()) == "" {
		return ErrEmptyTitleForSlug
	}

	// generate ID if not provided. Identity uniqueness is checked against the
	// in-memory index (s.tikis), not the filesystem — a tiki loaded from a
	// renamed file occupies an id without occupying <dir>/<id>.md, so an
	// os.Stat probe would falsely report the id free.
	if tk.ID() == "" {
		for i := 0; ; i++ {
			candidate := normalizeTikiID(config.GenerateRandomID())
			if _, taken := s.tikis[candidate]; !taken {
				tk.SetID(candidate)
				break
			}
			if i > maxGenerateRetries {
				return fmt.Errorf("failed to generate unique tiki id after %d attempts", maxGenerateRetries)
			}
			slog.Debug("ID collision detected in index, regenerating", "id", candidate)
		}
	} else {
		tk.SetID(normalizeTikiID(tk.ID()))
	}

	if err := s.validateTikiIDListFieldsLocked(tk); err != nil {
		return err
	}

	s.tikis[tk.ID()] = tk
	if err := s.saveTiki(tk); err != nil {
		delete(s.tikis, tk.ID())
		slog.Error("failed to save new document after creation", "tiki_id", tk.ID(), "error", err)
		return fmt.Errorf("failed to save tiki: %w", err)
	}
	return nil
}

// maxGenerateRetries caps the id-generation retry loop so a pathological
// index (e.g. someone wedged the whole 36^6 space) surfaces an error
// instead of looping forever.
const maxGenerateRetries = 100

// GetAllTikis returns every loaded tiki, including plain docs. The result is
// sorted by ID so callers see a deterministic order regardless of Go's
// randomized map iteration. Stable downstream sorts (e.g. ruki "order by"
// with tied keys) rely on this to keep tied tikis from swapping between
// renders.
func (s *TikiStore) GetAllTikis() []*tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*tikipkg.Tiki, 0, len(s.tikis))
	for _, tk := range s.tikis {
		out = append(out, tk)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// UpdateTiki updates an existing tiki and saves it. The incoming tiki's field
// set is authoritative (exact presence): fields absent in tk are not carried
// forward from the stored tiki, so deleted fields stay deleted on disk.
//
// Tiki-shaped callers that need carry-forward semantics (i.e. a partial Tiki
// that only sets a few fields) go through UpdateTiki, which performs the
// carry-forward merge before projecting to a Tiki and calling UpdateTiki.
func (s *TikiStore) UpdateTiki(tk *tikipkg.Tiki) error {
	if err := s.updateTikiCore(tk, false); err != nil {
		return err
	}
	slog.Info("tiki updated", "tiki_id", tk.ID())
	s.notifyListeners()
	return nil
}

// updateTikiCore persists an updated tiki. Path and LoadedMtime are carried
// forward from the stored tiki when absent on the incoming tiki (same as the
// pre-Phase-5 FilePath/LoadedMtime carry-forward). The field map is used as-is:
// no workflow-declared fields are injected or merged from the stored tiki.
func (s *TikiStore) updateTikiCore(tk *tikipkg.Tiki, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tk.SetID(normalizeTikiID(tk.ID()))
	old, exists := s.tikis[tk.ID()]
	if !exists {
		return fmt.Errorf("tiki not found: %s", tk.ID())
	}

	if tk.Path() == "" {
		tk.SetPath(old.Path())
	}
	if tk.LoadedMtime.IsZero() {
		tk.LoadedMtime = old.LoadedMtime
	}

	if err := s.validateTikiIDListFieldsLocked(tk); err != nil {
		return err
	}

	s.tikis[tk.ID()] = tk
	if err := s.saveTiki(tk); err != nil {
		s.tikis[tk.ID()] = old
		slog.Error("failed to save updated tiki", "tiki_id", tk.ID(), "error", err)
		return fmt.Errorf("failed to save tiki: %w", err)
	}
	return nil
}

// DeleteTiki removes a tiki and its file.
func (s *TikiStore) DeleteTiki(id string) {
	normalizedID := normalizeTikiID(id)
	if !s.deleteTikiLocked(normalizedID) {
		return
	}
	slog.Info("tiki deleted", "tiki_id", normalizedID)
	s.notifyListeners()
}

// deleteTikiLocked removes the tiki file and in-memory entry.
// Returns true if the tiki was deleted.
func (s *TikiStore) deleteTikiLocked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tikis[id]
	if !exists {
		return false
	}

	// use the loaded file path so a renamed/moved file still gets cleaned up.
	// existing came out of s.tikis, so it always has a Path and pathForTiki
	// short-circuits before any slug logic — the error branch is defensive.
	path, err := s.pathForTiki(existing)
	if err != nil {
		slog.Error("failed to resolve path for delete, tiki preserved", "tiki_id", id, "error", err)
		return false
	}

	// git integration is read-only: delete only the working-tree file, never
	// stage the removal. When cwd is a repo the user stages/commits themselves.
	if err := os.Remove(path); err != nil {
		slog.Error("file deletion failed, tiki preserved in memory", "tiki_id", id, "path", path, "error", err)
		return false
	}

	delete(s.tikis, id)
	return true
}

// validateTikiIDListFieldsLocked checks that references in every workflow-
// declared tikiIdList field resolve to loaded documents. References may target
// plain or workflow documents. Caller must hold s.mu lock.
func (s *TikiStore) validateTikiIDListFieldsLocked(tk *tikipkg.Tiki) error {
	for _, field := range workflow.WorkflowFields() {
		if field.Type != workflow.TypeListRef {
			continue
		}
		references, _, _ := tk.StringSliceField(field.Name)
		for _, referenceID := range references {
			normalized := normalizeTikiID(referenceID)
			if !idfmt.IsValidID(normalized) {
				return fmt.Errorf("%s reference %q is not a bare document id (expected %d uppercase alphanumeric chars)",
					field.Name, normalized, idfmt.IDLength)
			}
			if _, exists := s.tikis[normalized]; !exists {
				return fmt.Errorf("%s references non-existent document: %s", field.Name, normalized)
			}
		}
	}
	return nil
}
