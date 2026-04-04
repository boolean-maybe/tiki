package runtime

import (
	"fmt"
	"io"
	"strings"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/store"
)

// RunQuery parses and executes a ruki statement against the given store,
// writing formatted results to out. SELECT and UPDATE are supported;
// CREATE and DELETE are rejected.
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

	default:
		return fmt.Errorf("unsupported statement type")
	}
}

// RunSelectQuery is the legacy entry point. It delegates to RunQuery.
func RunSelectQuery(taskStore store.Store, query string, out io.Writer) error {
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

// resolveUser returns the current user name from the store.
// Returns an error if the user cannot be determined.
func resolveUser(s store.Store) (string, error) {
	name, _, err := s.GetCurrentUser()
	if err != nil {
		return "", err
	}
	return name, nil
}
