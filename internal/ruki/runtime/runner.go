package runtime

import (
	"fmt"
	"io"
	"strings"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/store"
)

// RunSelectQuery parses and executes a ruki select statement against the given
// store, writing formatted results to out. Non-select statements are rejected.
func RunSelectQuery(taskStore store.Store, query string, out io.Writer) error {
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

	if stmt.Select == nil {
		return fmt.Errorf("only select statements are supported")
	}

	tasks := taskStore.GetAllTasks()
	result, err := executor.Execute(stmt, tasks)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	formatter := NewTableFormatter()
	return formatter.Format(out, result.Select)
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
