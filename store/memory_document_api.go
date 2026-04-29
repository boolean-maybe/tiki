package store

import (
	"errors"
	"sort"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/task"
)

// errNilDocumentMemory is the InMemoryStore counterpart to
// tikistore.errNilDocument. Kept package-private because the two stores
// never share a sentinel — callers check errors.Is against the interface
// error returned by the specific method, not a shared type.
var errNilDocumentMemory = errors.New("document is nil")

// GetDocument returns the document with the given id regardless of
// workflow participation; plain docs are visible through this surface even
// though GetTask filters them.
func (s *InMemoryStore) GetDocument(id string) *document.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[normalizeTaskID(id)]
	if !ok {
		return nil
	}
	return task.ToDocument(t)
}

// GetAllDocuments returns every stored document in id order, including
// plain docs.
func (s *InMemoryStore) GetAllDocuments() []*document.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]*document.Document, 0, len(s.tasks))
	for _, t := range s.tasks {
		if d := task.ToDocument(t); d != nil {
			docs = append(docs, d)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs
}

// NewDocumentTemplate wraps NewTaskTemplate so document-first callers see
// the same workflow creation defaults.
func (s *InMemoryStore) NewDocumentTemplate() (*document.Document, error) {
	t, err := s.NewTaskTemplate()
	if err != nil {
		return nil, err
	}
	return task.ToDocument(t), nil
}

// ReloadDocument is a no-op for the in-memory store; present so the store
// satisfies DocumentReadStore. See ReloadTask for the rationale.
func (s *InMemoryStore) ReloadDocument(id string) error {
	return s.ReloadTask(id)
}

// CreateDocument adds a new document. Workflow-field presence in the
// source's Frontmatter decides whether the resulting store entry is a
// workflow item; plain docs stay plain because we bypass CreateTask's
// unconditional promotion.
func (s *InMemoryStore) CreateDocument(doc *document.Document) error {
	if doc == nil {
		return errNilDocumentMemory
	}
	t := task.FromDocument(doc)
	if err := s.storeNewDocumentLocked(t); err != nil {
		return err
	}
	doc.ID = t.ID
	return nil
}

// UpdateDocument updates an existing document. Unlike UpdateTask, the
// document-first path honors the presence-derived IsWorkflow that
// FromDocument computes: stripping every workflow key from the frontmatter
// and calling UpdateDocument demotes a workflow document to plain.
func (s *InMemoryStore) UpdateDocument(doc *document.Document) error {
	if doc == nil {
		return errNilDocumentMemory
	}
	return s.updateLocked(task.FromDocument(doc), false)
}

// DeleteDocument removes a document from the in-memory store.
func (s *InMemoryStore) DeleteDocument(id string) {
	s.DeleteTask(id)
}

// ensure InMemoryStore implements the Phase 4 document-first contract.
var _ DocumentStore = (*InMemoryStore)(nil)
