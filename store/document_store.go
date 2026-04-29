package store

import (
	"github.com/boolean-maybe/tiki/document"
)

// DocumentReadStore is the Phase 4 document-first read surface. It is the
// document-neutral counterpart to ReadStore: callers that want to browse or
// query all managed documents (plain markdown + workflow items) use this
// interface, while callers that must stay on the task-shaped API keep
// depending on ReadStore.
//
// Both interfaces project the same in-memory state; DocumentReadStore gives
// the unfiltered view (plain docs included), ReadStore gives the workflow-
// only view (presence-aware filter applied). A store implementation may
// satisfy both at once, which is the path Phase 4 takes: TikiStore and
// InMemoryStore are DocumentStore AND Store.
type DocumentReadStore interface {
	// GetDocument retrieves a document by ID. Returns nil when the ID is
	// not loaded. Unlike GetTask, this does not filter plain docs — any
	// loaded document is returned.
	GetDocument(id string) *document.Document

	// GetAllDocuments returns every loaded document, plain and workflow
	// alike. Contrast with GetAllTasks which applies the presence-aware
	// workflow filter.
	GetAllDocuments() []*document.Document

	// NewDocumentTemplate returns a new document populated with creation
	// defaults. Phase 4 delegates to NewTaskTemplate so workflow creation
	// defaults stay consistent; later phases may introduce plain-document
	// templates that do not promote to workflow.
	NewDocumentTemplate() (*document.Document, error)

	// ReloadDocument reloads a single document from disk by ID. Mirrors
	// ReloadTask but operates on the document-neutral surface.
	ReloadDocument(id string) error

	// PathForID returns the on-disk path of the document with the given id,
	// or the empty string when the id is unknown to the store. Identical
	// semantics to ReadStore.PathForID — the method is repeated here so
	// document-only consumers do not need to depend on ReadStore.
	PathForID(id string) string
}

// DocumentStore is the document-first full store surface, equivalent to
// Store but operating on *document.Document. Compatibility task methods
// continue to be served by Store; implementations typically satisfy both
// interfaces so callers migrate at their own pace.
type DocumentStore interface {
	DocumentReadStore

	// CreateDocument adds a new document to the store. Callers can omit the
	// ID to have one generated; the store normalizes it to uppercase.
	//
	// Workflow-field presence in Frontmatter decides whether the resulting
	// document participates in workflow: a Frontmatter map carrying any
	// workflow key (status, type, priority, points, tags, dependsOn, due,
	// recurrence, assignee) promotes the document to workflow-capable.
	CreateDocument(doc *document.Document) error

	// UpdateDocument updates an existing document. The document's Path is
	// authoritative for file operations; callers must not reconstruct the
	// path from the ID. Optimistic locking uses LoadedMtime as the compare
	// key, same as UpdateTask.
	UpdateDocument(doc *document.Document) error

	// DeleteDocument removes a document from the store.
	DeleteDocument(id string)
}
