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

// allDocsAsTasks returns every loaded document projected as a Task, including
// plain documents that GetAllTasks filters out.
//
// Phase 5 requires ruki `select`, `update`, and `delete` to operate across the
// full `.doc/` tree: `select where has(status)` must be able to see plain
// docs in order to exclude them, and `update set status = …` must be able to
// promote them. GetAllTasks applies the workflow-only filter at the store
// boundary, so the CLI would never see plain docs unless we bypass it.
//
// Implementation: prefer DocumentReadStore.GetAllDocuments (the unfiltered
// view) and project each document via task.FromDocument, which preserves
// FilePath, LoadedMtime, and WorkflowFrontmatter. These fields are what
// updateTaskLocked uses to carry forward identity-bound state when a fresh
// Task value lands in the store, so persistence through the gate still picks
// the right file and enforces optimistic locking.
//
// Fallback to GetAllTasks for ReadStore implementations that predate the
// document-neutral API (tests using bare mocks). Those callers simply retain
// the old workflow-only behavior.
func allDocsAsTasks(rs store.ReadStore) []*task.Task {
	if docStore, ok := rs.(store.DocumentReadStore); ok {
		docs := docStore.GetAllDocuments()
		tasks := make([]*task.Task, 0, len(docs))
		for _, d := range docs {
			if t := task.FromDocument(d); t != nil {
				tasks = append(tasks, t)
			}
		}
		return tasks
	}
	return rs.GetAllTasks()
}

// OutputFormat selects the renderer for CLI query output. OutputTable is the
// default text/table form; OutputJSON emits compact machine-readable JSON.
type OutputFormat int

const (
	// OutputTable renders human-readable text: ASCII table for selects, plain
	// text for scalars, and short English sentences for mutation summaries.
	OutputTable OutputFormat = iota
	// OutputJSON renders compact JSON: array of row objects for selects, bare
	// JSON values for scalars, and small summary objects for mutations/pipes.
	OutputJSON
)

// RunQueryOptions tunes CLI query execution. Zero value means table output
// and the real system clipboard, matching the default RunQuery behavior.
type RunQueryOptions struct {
	OutputFormat OutputFormat
	// ClipboardWriter is injected so tests can substitute a fake that doesn't
	// require a GUI clipboard binary (xclip/xsel/pbcopy). nil → system clipboard.
	ClipboardWriter func([][]string) error
}

// RunQuery parses and executes a ruki statement against the given gate,
// writing formatted results to out in the default table/text form.
func RunQuery(gate *service.TaskMutationGate, query string, out io.Writer) error {
	return RunQueryWithOptions(gate, query, out, RunQueryOptions{OutputFormat: OutputTable})
}

// RunQueryWithOptions is the options-aware entry point used by `tiki exec`.
// Callers that want the default text/table output should use RunQuery.
func RunQueryWithOptions(gate *service.TaskMutationGate, query string, out io.Writer, opts RunQueryOptions) error {
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

	tasks := allDocsAsTasks(readStore)
	result, err := executor.Execute(stmt, tasks, input)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	ctx := context.Background()
	json := opts.OutputFormat == OutputJSON

	switch {
	case result.Select != nil:
		return selectFormatter(json).Format(out, result.Select)

	case result.Update != nil:
		return persistAndSummarize(ctx, gate, result.Update, out, json)

	case result.Create != nil:
		return persistCreate(ctx, gate, result.Create, out, json)

	case result.Delete != nil:
		return persistDelete(ctx, gate, result.Delete, out, json)

	case result.Pipe != nil:
		return executePipe(ctx, result.Pipe, out, json)

	case result.Clipboard != nil:
		writer := opts.ClipboardWriter
		if writer == nil {
			writer = service.ExecuteClipboardPipe
		}
		if err := writer(result.Clipboard.Rows); err != nil {
			return err
		}
		return formatClipboardSummary(out, len(result.Clipboard.Rows), json)

	case result.Scalar != nil:
		if json {
			return FormatScalarJSON(out, result.Scalar)
		}
		return FormatScalar(out, result.Scalar)

	default:
		return fmt.Errorf("unsupported statement type")
	}
}

// selectFormatter returns the JSON or table formatter for select results.
func selectFormatter(json bool) Formatter {
	if json {
		return NewJSONFormatter()
	}
	return NewTableFormatter()
}

// executePipe runs the pipe command for each row. If any row fails, the first
// error is returned after all rows are attempted, matching the plugin-action
// behavior in controller/plugin.go where per-row failures log but don't abort.
func executePipe(ctx context.Context, pr *ruki.PipeResult, out io.Writer, json bool) error {
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
	return formatPipeSummary(out, succeeded, json)
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

	tasks := allDocsAsTasks(readStore)
	result, err := executor.Execute(validated, tasks, ruki.ExecutionInput{})
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	formatter := NewTableFormatter()
	return formatter.Format(out, result.Select)
}

func persistAndSummarize(ctx context.Context, gate *service.TaskMutationGate, ur *ruki.UpdateResult, out io.Writer, json bool) error {
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

	if werr := formatUpdateSummary(out, succeeded, failed, json); werr != nil {
		return werr
	}

	if failed > 0 {
		return fmt.Errorf("update partially failed: %d of %d tasks failed: %w", failed, succeeded+failed, firstErr)
	}
	return nil
}

func persistCreate(ctx context.Context, gate *service.TaskMutationGate, cr *ruki.CreateResult, out io.Writer, json bool) error {
	t := cr.Task

	if err := gate.CreateTask(ctx, t); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	return formatCreateSummary(out, t.ID, json)
}

func persistDelete(ctx context.Context, gate *service.TaskMutationGate, dr *ruki.DeleteResult, out io.Writer, json bool) error {
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

	if werr := formatDeleteSummary(out, succeeded, failed, json); werr != nil {
		return werr
	}

	if failed > 0 {
		return fmt.Errorf("delete partially failed: %d of %d tasks failed", failed, succeeded+failed)
	}
	return nil
}

// resolveUserFunc returns a userFunc closure for the executor, and an error
// if user resolution failed unexpectedly (as opposed to being deliberately
// unavailable). Delegates to store.CurrentUserDisplayFunc so CLI exec,
// plugin actions, triggers, and pipe-create setup all share the same rule.
func resolveUserFunc(s store.ReadStore) (func() string, error) {
	return store.CurrentUserDisplayFunc(s)
}
