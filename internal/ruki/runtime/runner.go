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
	"github.com/boolean-maybe/tiki/tiki"
)

// allDocsAsTikis returns every loaded tiki for ruki execution, including plain
// docs. Prefers rs.GetAllTikis() directly to avoid the Document→Task→Tiki
// roundtrip that drops tiki-native fields (e.g. CreatedBy, Body).
func allDocsAsTikis(rs store.ReadStore) []*tiki.Tiki {
	return rs.GetAllTikis()
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
		input.CreateTemplate = tiki.FromTask(template)
	}

	tikis := allDocsAsTikis(readStore)
	result, err := executor.Execute(stmt, tikis, input)
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

	tikis := allDocsAsTikis(readStore)
	result, err := executor.Execute(validated, tikis, ruki.ExecutionInput{})
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	formatter := NewTableFormatter()
	return formatter.Format(out, result.Select)
}

func persistAndSummarize(ctx context.Context, gate *service.TaskMutationGate, ur *ruki.UpdateResult, out io.Writer, json bool) error {
	var succeeded, failed int
	var firstErr error

	for _, tk := range ur.Updated {
		if err := gate.UpdateTiki(ctx, tk); err != nil {
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
	t := tiki.ToTask(cr.Tiki)

	if err := gate.CreateTask(ctx, t); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	return formatCreateSummary(out, t.ID, json)
}

func persistDelete(ctx context.Context, gate *service.TaskMutationGate, dr *ruki.DeleteResult, out io.Writer, json bool) error {
	readStore := gate.ReadStore()
	var succeeded, failed int
	for _, tk := range dr.Deleted {
		t := tiki.ToTask(tk)
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
