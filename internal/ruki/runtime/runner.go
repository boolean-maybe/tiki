package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// RunQuery parses and executes a ruki statement against the given gate,
// writing formatted results to out.
func RunQuery(gate *service.TaskMutationGate, query string, out io.Writer) error {
	query = strings.TrimSuffix(strings.TrimSpace(query), ";")
	if query == "" {
		return fmt.Errorf("empty query")
	}

	readStore := gate.ReadStore()

	schema := NewSchema()
	parser := ruki.NewParser(schema)

	userFunc, err := resolveUserFunc(readStore)
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}
	executor := ruki.NewExecutor(schema, userFunc, ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimeCLI})

	stmt, err := parser.ParseAndValidateStatement(query, ruki.ExecutorRuntimeCLI)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// for CREATE, fetch template before execution so field references
	// (e.g. tags=tags+["new"]) resolve from template defaults
	var input ruki.ExecutionInput
	if stmt.RequiresCreateTemplate() {
		var template *task.Task
		template, err = readStore.NewTaskTemplate()
		if err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		if template == nil {
			return fmt.Errorf("create template: store returned nil template")
		}
		input.CreateTemplate = template
	}

	tasks := readStore.GetAllTasks()
	result, err := executor.Execute(stmt, tasks, input)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	ctx := context.Background()

	switch {
	case result.Select != nil:
		formatter := NewTableFormatter()
		return formatter.Format(out, result.Select)

	case result.Update != nil:
		return persistAndSummarize(ctx, gate, result.Update, out)

	case result.Create != nil:
		return persistCreate(ctx, gate, result.Create, out)

	case result.Delete != nil:
		return persistDelete(ctx, gate, result.Delete, out)

	case result.Pipe != nil:
		return executePipe(ctx, result.Pipe, out)

	case result.Clipboard != nil:
		if err := service.ExecuteClipboardPipe(result.Clipboard.Rows); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "copied %d rows to clipboard\n", len(result.Clipboard.Rows))
		return nil

	case result.Scalar != nil:
		return FormatScalar(out, result.Scalar)

	default:
		return fmt.Errorf("unsupported statement type")
	}
}

// executePipe runs the pipe command for each row. If any row fails, the first
// error is returned after all rows are attempted, matching the plugin-action
// behavior in controller/plugin.go where per-row failures log but don't abort.
func executePipe(ctx context.Context, pr *ruki.PipeResult, out io.Writer) error {
	var firstErr error
	succeeded := 0
	for _, row := range pr.Rows {
		if err := service.ExecutePipeCommand(ctx, pr.Command, row); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		succeeded++
	}
	if firstErr != nil {
		return firstErr
	}
	_, _ = fmt.Fprintf(out, "ran command on %d rows\n", succeeded)
	return nil
}

// RunSelectQuery is the read-only entry point restricted to SELECT statements.
// Non-SELECT statements (CREATE, UPDATE, DELETE) are rejected to preserve
// read-only semantics expected by callers of this function.
func RunSelectQuery(readStore store.ReadStore, query string, out io.Writer) error {
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
	validated, err := ruki.NewSemanticValidator(ruki.ExecutorRuntimeCLI).ValidateStatement(stmt)
	if err != nil {
		return fmt.Errorf("semantic validate: %w", err)
	}

	userFunc, err := resolveUserFunc(readStore)
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}
	executor := ruki.NewExecutor(schema, userFunc, ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimeCLI})

	tasks := readStore.GetAllTasks()
	result, err := executor.Execute(validated, tasks, ruki.ExecutionInput{})
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	formatter := NewTableFormatter()
	return formatter.Format(out, result.Select)
}

func persistAndSummarize(ctx context.Context, gate *service.TaskMutationGate, ur *ruki.UpdateResult, out io.Writer) error {
	var succeeded, failed int
	var firstErr error

	for _, t := range ur.Updated {
		if err := gate.UpdateTask(ctx, t); err != nil {
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

func persistCreate(ctx context.Context, gate *service.TaskMutationGate, cr *ruki.CreateResult, out io.Writer) error {
	t := cr.Task

	if err := gate.CreateTask(ctx, t); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	_, _ = fmt.Fprintf(out, "created %s\n", t.ID)
	return nil
}

func persistDelete(ctx context.Context, gate *service.TaskMutationGate, dr *ruki.DeleteResult, out io.Writer) error {
	readStore := gate.ReadStore()
	var succeeded, failed int
	for _, t := range dr.Deleted {
		if err := gate.DeleteTask(ctx, t); err != nil {
			failed++
		} else if readStore.GetTask(t.ID) != nil {
			// store silently failed to delete
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

// resolveUserFunc returns a userFunc closure for the executor, and an error
// if user resolution failed unexpectedly (as opposed to being deliberately
// unavailable). Returns nil userFunc when user is empty (git disabled),
// which causes user() calls in queries to return a clear error.
func resolveUserFunc(s store.ReadStore) (func() string, error) {
	name, _, err := s.GetCurrentUser()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, nil
	}
	return func() string { return name }, nil
}
