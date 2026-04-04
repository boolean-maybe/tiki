package runtime

import (
	"fmt"
	"io"
	"strings"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// RunQuery parses and executes a ruki statement against the given store,
// writing formatted results to out.
func RunQuery(taskStore store.Store, query string, out io.Writer) error {
	query = strings.TrimSuffix(strings.TrimSpace(query), ";")
	if query == "" {
		return fmt.Errorf("empty query")
	}

	schema := NewSchema()
	parser := ruki.NewParser(schema)

	userName, err := resolveUser(taskStore)
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}
	executor := ruki.NewExecutor(schema, func() string { return userName })

	stmt, err := parser.ParseStatement(query)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// for CREATE, fetch template before execution so field references
	// (e.g. tags=tags+["new"]) resolve from template defaults
	var template *task.Task
	if stmt.Create != nil {
		template, err = taskStore.NewTaskTemplate()
		if err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		if template == nil {
			return fmt.Errorf("create template: store returned nil template")
		}
		executor.SetTemplate(template)
	}

	tasks := taskStore.GetAllTasks()
	result, err := executor.Execute(stmt, tasks)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	switch {
	case result.Select != nil:
		formatter := NewTableFormatter()
		return formatter.Format(out, result.Select)

	case result.Update != nil:
		return persistAndSummarize(taskStore, result.Update, out)

	case result.Create != nil:
		return persistCreate(taskStore, result.Create, template, out)

	case result.Delete != nil:
		return persistDelete(taskStore, result.Delete, out)

	default:
		return fmt.Errorf("unsupported statement type")
	}
}

// RunSelectQuery is the legacy entry point restricted to SELECT statements.
// Non-SELECT statements (CREATE, UPDATE, DELETE) are rejected to preserve
// read-only semantics expected by callers of this function.
func RunSelectQuery(taskStore store.Store, query string, out io.Writer) error {
	trimmed := strings.TrimSuffix(strings.TrimSpace(query), ";")
	if trimmed == "" {
		return fmt.Errorf("empty query")
	}

	schema := NewSchema()
	parser := ruki.NewParser(schema)
	stmt, err := parser.ParseStatement(trimmed)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if stmt.Select == nil {
		return fmt.Errorf("RunSelectQuery only supports SELECT statements")
	}

	return RunQuery(taskStore, query, out)
}

func persistAndSummarize(taskStore store.Store, ur *ruki.UpdateResult, out io.Writer) error {
	var succeeded, failed int
	var firstErr error

	for _, t := range ur.Updated {
		if err := taskStore.UpdateTask(t); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
		}
	}

	if failed > 0 {
		_, _ = fmt.Fprintf(out, "updated %d tasks (%d failed)\n", succeeded, failed)
		return fmt.Errorf("update partially failed: %d of %d tasks failed: %w", failed, succeeded+failed, firstErr)
	}

	_, _ = fmt.Fprintf(out, "updated %d tasks\n", succeeded)
	return nil
}

func persistCreate(taskStore store.Store, cr *ruki.CreateResult, template *task.Task, out io.Writer) error {
	t := cr.Task
	t.ID = template.ID
	t.CreatedBy = template.CreatedBy
	t.CreatedAt = template.CreatedAt

	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("create requires a title")
	}

	if err := taskStore.CreateTask(t); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, _ = fmt.Fprintf(out, "created %s\n", t.ID)
	return nil
}

func persistDelete(taskStore store.Store, dr *ruki.DeleteResult, out io.Writer) error {
	var succeeded, failed int
	for _, t := range dr.Deleted {
		taskStore.DeleteTask(t.ID)
		if taskStore.GetTask(t.ID) != nil {
			failed++
		} else {
			succeeded++
		}
	}

	if failed > 0 {
		_, _ = fmt.Fprintf(out, "deleted %d tasks (%d failed)\n", succeeded, failed)
		return fmt.Errorf("delete partially failed: %d of %d tasks failed", failed, succeeded+failed)
	}

	_, _ = fmt.Fprintf(out, "deleted %d tasks\n", succeeded)
	return nil
}

// resolveUser returns the current user name from the store.
// Returns an error if the user cannot be determined.
func resolveUser(s store.Store) (string, error) {
	name, _, err := s.GetCurrentUser()
	if err != nil {
		return "", err
	}
	return name, nil
}
