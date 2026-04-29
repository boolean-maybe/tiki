package tikistore

import (
	"errors"
	"log/slog"
	"sort"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// errNilDocument is returned when a document-first mutation method is
// called with a nil document. Kept as a sentinel so callers can match on
// it without string comparison.
var errNilDocument = errors.New("document is nil")

// GetDocument returns the document with the given id, plain or workflow.
// Unlike GetTask, this does not apply the workflow presence filter — any
// loaded document is returned. Returns nil when the id is unknown.
func (s *TikiStore) GetDocument(id string) *document.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[normalizeTaskID(id)]
	if !ok {
		return nil
	}
	return taskpkg.ToDocument(t)
}

// GetAllDocuments returns every loaded document in deterministic id order,
// including plain docs that GetAllTasks filters out. Callers that want only
// workflow documents should continue using GetAllTasks (or filter the
// returned slice by document.IsWorkflowFrontmatter).
func (s *TikiStore) GetAllDocuments() []*document.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]*document.Document, 0, len(s.tasks))
	for _, t := range s.tasks {
		if d := taskpkg.ToDocument(t); d != nil {
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
	if err := s.storeNewDocumentLocked(t); err != nil {
		return err
	}
	slog.Info("document created", "task_id", t.ID, "workflow", t.IsWorkflow)
	s.notifyListeners()
	// Write the adapter's normalized id back onto the caller's document so
	// callers that inspect doc.ID after create see the same value the store
	// keyed off.
	doc.ID = t.ID
	return nil
}

// UpdateDocument updates an existing document. Path/LoadedMtime on the
// Document are carried through to the underlying Task so optimistic locking
// and rename-preservation keep working.
//
// Unlike UpdateTask, the document-first path honors the presence-derived
// IsWorkflow that FromDocument computes: stripping every workflow key from
// the frontmatter and calling UpdateDocument demotes a workflow document
// back to a plain doc. UpdateTask keeps its protective carry-forward for
// task-shaped callers (ruki/UI) that legitimately forget to set IsWorkflow.
func (s *TikiStore) UpdateDocument(doc *document.Document) error {
	if doc == nil {
		return errNilDocument
	}
	t := taskpkg.FromDocument(doc)
	if err := s.updateTaskLocked(t, false); err != nil {
		return err
	}
	slog.Info("document updated", "task_id", t.ID, "workflow", t.IsWorkflow)
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
