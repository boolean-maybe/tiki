package tikistore

import (
	"errors"
	"log/slog"
	"sort"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// errNilDocument is returned when a document-first mutation method is
// called with a nil document. Kept as a sentinel so callers can match on
// it without string comparison.
var errNilDocument = errors.New("document is nil")

// GetDocument returns the document with the given id, plain or workflow.
// Unlike GetTask, this does not apply the workflow presence filter — any
// loaded document is returned. Returns nil when the id is unknown.
func (s *TikiStore) GetDocument(id string) *document.Document {
	tk := s.GetTiki(id)
	if tk == nil {
		return nil
	}
	return taskpkg.ToDocument(tikipkg.ToTask(tk))
}

// GetAllDocuments returns every loaded document in deterministic id order,
// including plain docs. To restrict to workflow documents, filter the returned
// slice by document.IsWorkflowFrontmatter or use GetAllTikis with hasAnyWorkflowField.
func (s *TikiStore) GetAllDocuments() []*document.Document {
	tikis := s.GetAllTikis()
	docs := make([]*document.Document, 0, len(tikis))
	for _, tk := range tikis {
		if d := taskpkg.ToDocument(tikipkg.ToTask(tk)); d != nil {
			docs = append(docs, d)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs
}

// NewDocumentTemplate returns a new document seeded with workflow creation
// defaults. Phase 4 delegates to NewTaskTemplate — the underlying template
// IS a workflow item, which matches the plan's rule that workflow/ruki
// creation paths are the only ones that default-promote.
func (s *TikiStore) NewDocumentTemplate() (*document.Document, error) {
	t, err := s.NewTaskTemplate()
	if err != nil {
		return nil, err
	}
	return taskpkg.ToDocument(t), nil
}

// ReloadDocument reloads a single document from disk by ID. Delegates to
// ReloadTask; the two share an implementation because the id-indexed in-
// memory map is the same.
func (s *TikiStore) ReloadDocument(id string) error {
	return s.ReloadTask(id)
}

// CreateDocument adds a new document. The source's Frontmatter determines
// workflow participation: a document carrying any workflow key is
// workflow-capable, a document without any is plain. The on-disk YAML
// matches that classification (see marshalFrontmatter), so the file
// survives a reload without accidental promotion.
func (s *TikiStore) CreateDocument(doc *document.Document) error {
	if doc == nil {
		return errNilDocument
	}
	t := taskpkg.FromDocument(doc)
	tk := tikipkg.FromTask(t)
	if err := s.storeNewDocumentLocked(tk); err != nil {
		return err
	}
	slog.Info("document created", "task_id", tk.ID, "workflow", hasAnyWorkflowField(tk))
	s.notifyListeners()
	// Write the adapter's normalized id back onto the caller's document so
	// callers that inspect doc.ID after create see the same value the store
	// keyed off.
	doc.ID = tk.ID
	return nil
}

// UpdateDocument updates an existing document. Path/LoadedMtime on the
// Document are carried through to the underlying Tiki so optimistic locking
// and rename-preservation keep working.
//
// Unlike UpdateTask, the document-first path honors exact field presence:
// stripping every workflow key from the frontmatter demotes a workflow
// document back to a plain doc. UpdateTask keeps its protective carry-forward
// for task-shaped callers (ruki/UI) that legitimately omit workflow fields
// when rebuilding a task from a subset of fields.
func (s *TikiStore) UpdateDocument(doc *document.Document) error {
	if doc == nil {
		return errNilDocument
	}
	t := taskpkg.FromDocument(doc)
	tk := tikipkg.FromTask(t)
	// UpdateTiki uses exact presence: absent fields in tk are not carried
	// forward from the stored tiki, so explicit key removal is honored and
	// full demotion (stripping all workflow keys) works correctly.
	if err := s.UpdateTiki(tk); err != nil {
		return err
	}
	slog.Info("document updated", "task_id", tk.ID, "workflow", hasAnyWorkflowField(tk))
	s.notifyListeners()
	return nil
}

// DeleteDocument removes a document; thin wrapper around DeleteTask because
// the underlying store key space is shared.
func (s *TikiStore) DeleteDocument(id string) {
	s.DeleteTask(id)
}

// ensure TikiStore implements the Phase 4 document-first contract.
var _ store.DocumentStore = (*TikiStore)(nil)
