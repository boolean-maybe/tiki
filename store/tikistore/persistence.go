package tikistore

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store/internal/git"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

// loadLocked reads all task files from the document root, scanning
// recursively so documents organized in subdirectories load alongside
// flat ones. Phase 2 generalizes the store from a single flat task
// directory to `.doc/**/*.md`; excluded files (config.yaml, workflow.yaml,
// non-markdown, hidden paths) are skipped and every loaded file must
// carry a valid bare frontmatter id.
//
// Caller must hold s.mu lock.
func (s *TikiStore) loadLocked() error {
	slog.Debug("loading documents from directory", "dir", s.dir)
	// create directory if it doesn't exist
	//nolint:gosec // G301: 0755 is appropriate for document storage directory
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		slog.Error("failed to create document directory", "dir", s.dir, "error", err)
		return fmt.Errorf("creating directory: %w", err)
	}

	docPaths, err := s.collectDocumentPaths()
	if err != nil {
		slog.Error("failed to walk document directory", "dir", s.dir, "error", err)
		return fmt.Errorf("walking directory: %w", err)
	}

	// Pre-fetch all author info in one batch git operation. The old flat-dir
	// code used a `<dir>/*.md` glob; with recursive loading we pass the
	// directory itself so git's pathspec walks every nested `.md` file. The
	// returned map is keyed by repo-relative path, matching both flat and
	// nested layouts identically.
	var authorMap map[string]*git.AuthorInfo
	var lastCommitMap map[string]time.Time
	if s.gitUtil != nil {
		if authors, err := s.gitUtil.AllAuthors(s.dir); err == nil {
			authorMap = authors
		} else {
			slog.Warn("failed to batch fetch authors", "error", err)
		}

		if lastCommits, err := s.gitUtil.AllLastCommitTimes(s.dir); err == nil {
			lastCommitMap = lastCommits
		} else {
			slog.Warn("failed to batch fetch last commit times", "error", err)
		}
	}

	// reset diagnostics for this load cycle so callers see a fresh report.
	s.diagnostics = newLoadDiagnostics()
	idIndex := document.NewIndex()
	for _, filePath := range docPaths {
		tk, err := s.loadTikiFile(filePath, authorMap, lastCommitMap)
		if err != nil {
			// classify via *loadError; fall back to LoadReasonOther.
			reason := LoadReasonOther
			var le *loadError
			if errors.As(err, &le) {
				reason = le.reason
			}
			s.diagnostics.record(filePath, reason, err.Error())
			slog.Error("failed to load task file", "file", filePath, "reason", reason, "error", err)
			continue
		}

		if err := idIndex.Register(tk.ID, filePath); err != nil {
			// duplicate id: refuse to silently pick a winner. Both files stay
			// on disk; run `tiki repair ids --fix --regenerate-duplicates` to
			// resolve.
			s.diagnostics.record(filePath, LoadReasonDuplicateID, err.Error())
			slog.Error("duplicate document id detected, skipping later occurrence",
				"task_id", tk.ID, "file", filePath, "error", err)
			continue
		}
		s.tikis[tk.ID] = tk
		slog.Debug("loaded task", "task_id", tk.ID, "file", filePath)
	}
	slog.Info("finished loading tasks", "num_tasks", len(s.tikis), "rejections", len(s.diagnostics.Rejections()))
	return nil
}

// loadTikiFile parses a single markdown file into a *tiki.Tiki. Returns a
// *loadError on identity/frontmatter failures so loadLocked can classify
// the rejection; unwrap via errors.As for the general case.
//
// Phase 5: the load path now terminates at *tiki.Tiki (no longer projects
// to *taskpkg.Task). Workflow defaults/validation that used to run on the
// Task projection now operate directly on the Tiki's Fields map.
func (s *TikiStore) loadTikiFile(path string, authorMap map[string]*git.AuthorInfo, lastCommitMap map[string]time.Time) (*tiki.Tiki, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, newLoadError(LoadReasonOther, fmt.Errorf("stat file: %w", err))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, newLoadError(LoadReasonOther, fmt.Errorf("reading file: %w", err))
	}

	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		slog.Debug("failed to resolve absolute task path, using raw path", "file", path, "error", absErr)
		absPath = path
	}

	parsed, err := loadTikiFromBytes(absPath, content)
	if err != nil {
		return nil, err
	}
	parsed.t.LoadedMtime = info.ModTime()

	// Type validation: workflow docs with an unknown explicit type are a
	// hard load error (matches pre-Phase-3 behavior). Plain docs have no
	// `type:` in their frontmatter so this check is a no-op for them.
	isWorkflow := document.IsWorkflowFrontmatter(parsed.raw)
	if isWorkflow {
		if rawType, ok := parsed.raw["type"].(string); ok && rawType != "" {
			if _, typeOK := taskpkg.ParseType(rawType); !typeOK {
				return nil, fmt.Errorf("unknown type %q", rawType)
			}
		} else if fmType, ok := parsed.t.Fields["type"].(string); ok && fmType != "" {
			// belt-and-suspenders — Fields["type"] set from taskFrontmatter.Type
			if _, typeOK := taskpkg.ParseType(fmType); !typeOK {
				return nil, fmt.Errorf("unknown type %q", fmType)
			}
		}
	}

	tk := parsed.t

	// Propagate the stale-provenance set from parsedTiki onto the Tiki so
	// downstream consumers (validateTikiCustomFields, StaleKeys, ToTask) can
	// distinguish registered Custom fields that failed coercion on load
	// (round-trip as unknown) from ones the caller explicitly wrote (validated
	// as Custom). parsedTiki.stale is the authoritative source; tiki.stale is
	// the unexported field exposed via markStale.
	for k := range parsed.stale {
		tk.MarkStaleForPersistence(k)
	}

	// Apply workflow validation / clamping on the Tiki's Fields map.
	// Matches the pre-Phase-3 behavior where only workflow docs got
	// Priority/Points clamping; plain docs keep their zero values.
	//
	// NOTE: We intentionally do NOT inject a default "type" here even for
	// workflow files that omit it. The tiki Fields map must faithfully
	// reflect what is on disk (sparse presence); applying a default type
	// would cause GetDocument to leak "type" into the frontmatter for files
	// that never had it, violating sparse-presence semantics.
	// The default type is applied at the task projection layer (ToTask / tikiToTask)
	// for display and query purposes without polluting the tiki's field map.
	if isWorkflow {
		if priority, ok := coerceIntForYAML(tk.Fields["priority"]); ok {
			if priority < taskpkg.MinPriority || priority > taskpkg.MaxPriority {
				slog.Debug("invalid priority value, using default",
					"task_id", tk.ID, "file", path, "invalid_value", priority,
					"default", taskpkg.DefaultPriority)
				tk.Set("priority", taskpkg.DefaultPriority)
			}
		}
		maxPoints := config.GetMaxPoints()
		if points, ok := coerceIntForYAML(tk.Fields["points"]); ok {
			if points < 1 || points > maxPoints {
				slog.Debug("invalid points value, using default",
					"task_id", tk.ID, "file", path, "invalid_value", points,
					"default", maxPoints/2)
				tk.Set("points", maxPoints/2)
			}
		}
	}

	// Compute UpdatedAt as max(file_mtime, last_git_commit_time)
	tk.UpdatedAt = info.ModTime()
	if lastCommitMap != nil {
		relPath := relPathForLookup(s.dir, path)
		if lastCommit, exists := lastCommitMap[relPath]; exists {
			if lastCommit.After(tk.UpdatedAt) {
				tk.UpdatedAt = lastCommit
			}
		}
	}

	// Populate CreatedAt/CreatedBy from author map (already fetched in batch).
	// CreatedBy is stored in Fields so ruki's identity-field reads work.
	if authorMap != nil {
		relPath := relPathForLookup(s.dir, path)
		if author, exists := authorMap[relPath]; exists {
			switch {
			case author.Name != "":
				tk.Set("createdBy", author.Name)
			case author.Email != "":
				tk.Set("createdBy", author.Email)
			}
			tk.CreatedAt = author.Date
		}
	}

	// Fallback to file metadata when git history is not available.
	if tk.CreatedAt.IsZero() {
		tk.CreatedAt = info.ModTime()
		if name, email, err := s.GetCurrentUser(); err == nil {
			switch {
			case name != "":
				tk.Set("createdBy", name)
			case email != "":
				tk.Set("createdBy", email)
			}
		}
	}

	s.upgrader.UpgradeTiki(tk)

	return tk, nil
}

// relPathForLookup returns the lookup key for the authorMap / lastCommitMap.
// Both maps are keyed by the path as constructed from s.dir (i.e. the
// already-joined relative path). When path is absolute, rebuild the key the
// same way AllAuthors/AllLastCommitTimes built it.
func relPathForLookup(dir, path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return filepath.Join(dir, rel)
}

// Reload reloads all tasks from disk
func (s *TikiStore) Reload() error {
	slog.Info("reloading tasks from disk")
	start := time.Now()
	s.mu.Lock()
	s.tikis = make(map[string]*tiki.Tiki)

	if err := s.loadLocked(); err != nil {
		s.mu.Unlock()
		slog.Error("error reloading tasks from disk", "error", err)
		return err
	}
	s.mu.Unlock()

	slog.Info("tasks reloaded successfully", "duration", time.Since(start).Round(time.Millisecond))
	s.notifyListeners()
	return nil
}

// ReloadTask reloads a single task from disk by ID.
// Uses the loaded tiki's Path so renamed files still reload from their
// current location. For IDs unknown to the store, falls back to the
// id-derived default path (used by CreateTask reload-after-save flows).
func (s *TikiStore) ReloadTask(taskID string) error {
	normalizedID := normalizeTaskID(taskID)
	slog.Debug("reloading single task", "task_id", normalizedID)

	s.mu.RLock()
	existing := s.tikis[normalizedID]
	s.mu.RUnlock()

	var filePath string
	if existing != nil && existing.Path != "" {
		filePath = existing.Path
	} else {
		filePath = s.taskFilePath(normalizedID)
	}

	// Fetch git info for this single file
	var authorMap map[string]*git.AuthorInfo
	var lastCommitMap map[string]time.Time
	if s.gitUtil != nil {
		if authors, err := s.gitUtil.AllAuthors(filePath); err == nil {
			authorMap = authors
		}
		if lastCommits, err := s.gitUtil.AllLastCommitTimes(filePath); err == nil {
			lastCommitMap = lastCommits
		}
	}

	// Load the tiki — if it's now invalid (e.g. bad type after external edit),
	// remove the stale in-memory copy rather than leaving it pretending the file is valid.
	tk, err := s.loadTikiFile(filePath, authorMap, lastCommitMap)
	if err != nil {
		s.mu.Lock()
		delete(s.tikis, normalizedID)
		s.mu.Unlock()
		slog.Warn("removed invalid task from memory after reload failure",
			"task_id", normalizedID, "file", filePath, "error", err)
		s.notifyListeners()
		return fmt.Errorf("loading task file %s: %w", filePath, err)
	}

	// Enforce the same id invariants the full load path enforces: a
	// ReloadTask must not leave stale entries behind when the file's
	// frontmatter id has changed, and must refuse to overwrite another
	// task's entry with this one.
	s.mu.Lock()
	if tk.ID != normalizedID {
		// id changed in the frontmatter (e.g. user edited it externally).
		// Drop the old map entry first — whatever the next branch does,
		// the old id is no longer backed by any file on disk and must not
		// linger in the index.
		delete(s.tikis, normalizedID)

		// Refuse to promote the new id if another loaded tiki already owns
		// it — that would silently overwrite the peer in memory while both
		// files sit on disk. Leave the new id un-indexed too: the file at
		// tk.Path still exists, but the store refuses to claim it until the
		// collision is resolved (e.g. via
		// `tiki repair ids --fix --regenerate-duplicates`). The next full
		// Reload will surface the duplicate through LoadDiagnostics.
		if peer, taken := s.tikis[tk.ID]; taken && peer.Path != tk.Path {
			s.mu.Unlock()
			slog.Error("refusing to reload: id change would collide with another task",
				"old_id", normalizedID, "new_id", tk.ID,
				"new_file", tk.Path, "existing_file", peer.Path)
			s.notifyListeners()
			return fmt.Errorf("reload: id change %s → %s collides with %s at %s",
				normalizedID, tk.ID, tk.ID, peer.Path)
		}
	}
	s.tikis[tk.ID] = tk
	s.mu.Unlock()

	s.notifyListeners()
	slog.Debug("task reloaded successfully", "task_id", tk.ID)
	return nil
}

// validateTikiCustomFields enforces the same save-time contract as
// validateCustomFields but operates on a *tiki.Tiki instead of task.Task.
// Custom fields are any Fields entries that are (a) not schema-known, (b)
// not identity/comments keys, (c) not marked stale, and (d) registered as
// Custom=true in the workflow registry. Non-custom registered fields and
// unregistered (unknown) fields bypass validation and round-trip verbatim.
func validateTikiCustomFields(tk *tiki.Tiki) error {
	if tk == nil || len(tk.Fields) == 0 {
		return nil
	}
	staleKeys := tk.StaleKeys()
	for k := range tk.Fields {
		if tiki.IsSchemaKnown(k) || k == "createdBy" || k == "comments" {
			continue
		}
		if _, isStale := staleKeys[k]; isStale {
			// stale = came from UnknownFields; bypass validation like Task.UnknownFields
			continue
		}
		fd, registered := workflow.Field(k)
		if !registered || !fd.Custom {
			// unregistered keys round-trip as unknown — not a custom field error
			continue
		}
		// it IS a registered custom field: validate it
		if err := validateCustomFieldEntry(k, tk.Fields[k], fd); err != nil {
			return err
		}
	}
	return nil
}

// validateCustomFieldEntry validates a single registered custom field value.
// Mirrors the list-shape check in validateCustomFields.
func validateCustomFieldEntry(k string, v interface{}, fd workflow.FieldDef) error {
	switch fd.Type {
	case workflow.TypeListString, workflow.TypeListRef:
		if _, err := coerceStringList(v); err != nil {
			return fmt.Errorf("invalid list value for %q: %w", k, err)
		}
	}
	return nil
}

// saveTiki writes a tiki to its markdown file. For loaded tikis the path
// follows tk.Path so renames are preserved; new tikis land at the
// id-derived default path.
func (s *TikiStore) saveTiki(tk *tiki.Tiki) error {
	path := s.pathForTiki(tk)
	slog.Debug("attempting to save task", "task_id", tk.ID, "path", path)

	// Validate custom fields before write: reject unregistered keys that
	// were staged under task.CustomFields. This preserves the pre-Phase-3
	// appendCustomFields contract; UnknownFields (stale-marked) bypass
	// the check by design.
	if err := validateTikiCustomFields(tk); err != nil {
		slog.Error("custom-field validation failed", "task_id", tk.ID, "error", err)
		return err
	}

	// Check for external modification (optimistic locking)
	// Only check if tiki was previously loaded (LoadedMtime is not zero)
	if !tk.LoadedMtime.IsZero() {
		if info, err := os.Stat(path); err == nil {
			if !info.ModTime().Equal(tk.LoadedMtime) {
				slog.Warn("task modified externally, conflict detected", "task_id", tk.ID, "path", path, "loaded_mtime", tk.LoadedMtime, "file_mtime", info.ModTime())
				return ErrConflict
			}
		} else if !os.IsNotExist(err) {
			slog.Error("failed to stat file for optimistic locking", "task_id", tk.ID, "path", path, "error", err)
			return fmt.Errorf("stat file for optimistic locking: %w", err)
		}
	}

	yamlBytes, err := marshalTikiFrontmatter(tk)
	if err != nil {
		slog.Error("failed to marshal frontmatter for task", "task_id", tk.ID, "error", err)
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var content strings.Builder
	content.WriteString("---\n")
	content.Write(yamlBytes)
	content.WriteString("---\n")
	if tk.Body != "" {
		content.WriteString(tk.Body)
		content.WriteString("\n")
	}

	// Ensure parent directory exists; with recursive loading a document's
	// Path may point into a subdirectory that hasn't been created on this
	// filesystem (e.g. a file dragged into a new folder on another machine
	// and replayed via git pull).
	//nolint:gosec // G301: 0755 matches the existing dir permissions
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Error("failed to create parent directory for document", "task_id", tk.ID, "path", path, "error", err)
		return fmt.Errorf("creating parent directory: %w", err)
	}

	//nolint:gosec // G306: 0644 is appropriate for document files
	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		slog.Error("failed to write task file", "task_id", tk.ID, "path", path, "error", err)
		return fmt.Errorf("writing file: %w", err)
	}

	// Update LoadedMtime and Path after successful save
	if info, err := os.Stat(path); err == nil {
		tk.LoadedMtime = info.ModTime()
		tk.UpdatedAt = info.ModTime()
		if s.gitUtil != nil {
			if lastCommit, err := s.gitUtil.LastCommitTime(path); err == nil {
				if lastCommit.After(tk.UpdatedAt) {
					tk.UpdatedAt = lastCommit
				}
			}
		}
		if absPath, absErr := filepath.Abs(path); absErr == nil {
			tk.Path = absPath
		} else {
			slog.Debug("failed to resolve absolute task path after save, using raw path", "task_id", tk.ID, "path", path, "error", absErr)
			tk.Path = path
		}
		slog.Debug("task file saved and timestamps computed", "task_id", tk.ID, "path", path, "new_mtime", tk.LoadedMtime, "updated_at", tk.UpdatedAt)
	} else {
		slog.Error("failed to stat file after save for mtime computation", "task_id", tk.ID, "path", path, "error", err)
	}

	// Git add the modified file (best effort)
	if s.gitUtil != nil {
		if err := s.gitUtil.Add(path); err != nil {
			slog.Warn("failed to git add task file", "task_id", tk.ID, "path", path, "error", err)
		}
	}

	slog.Info("task saved successfully", "task_id", tk.ID, "path", path)
	return nil
}

// taskFilePath returns the default on-disk path for a brand-new task whose
// file has not yet been created. Once a task is loaded from disk, save and
// delete operations should target task.FilePath — not this — so renames are
// preserved. In Phase 2 this resolves to `.doc/<ID>.md` because the store
// is rooted at the unified document directory.
func (s *TikiStore) taskFilePath(id string) string {
	return filepath.Join(s.dir, id+".md")
}

// collectDocumentPaths delegates to the shared document.WalkDocuments so
// the store and the `tiki repair` command classify "managed document" files
// identically. Centralizing the walk prevents drift between what the store
// refuses to load and what repair silently skips.
func (s *TikiStore) collectDocumentPaths() ([]string, error) {
	return document.WalkDocuments(s.dir)
}

// pathForTiki returns the on-disk path to use for an operation on tk.
// If tk already has a Path (was loaded from disk), that path wins so
// rename/move are preserved. Only when Path is empty (brand-new tiki
// being created) do we fall back to the id-derived path.
func (s *TikiStore) pathForTiki(tk *tiki.Tiki) string {
	if tk != nil && tk.Path != "" {
		return tk.Path
	}
	return s.taskFilePath(tk.ID)
}

// validateCustomFields enforces the save-time contract inherited from the
// pre-Phase-3 appendCustomFields helper:
//
//  1. every key in Task.CustomFields must be a registered Custom field;
//  2. for list-typed custom fields (TypeListString, TypeListRef), the
//     value must coerce to a string list — a scalar or wrong-shape value
//     fails the save with "invalid list value for %q: ...".
//
// Task.UnknownFields is the load-only slot for values that came off disk
// but didn't match a registered Custom field (or failed coercion); those
// bypass both checks by design so they round-trip verbatim for repair.
//
// Non-list custom types (enum, int, string, timestamp, bool) are not
// value-checked here because the emitter relies on yaml.Marshal to handle
// them and a type mismatch surfaces as a marshaling error downstream. The
// appendCustomFields path likewise only pre-validated list shapes.
func validateCustomFields(customFields map[string]interface{}) error {
	for k, v := range customFields {
		fd, ok := workflow.Field(k)
		if !ok || !fd.Custom {
			return fmt.Errorf("unknown custom field %q", k)
		}
		switch fd.Type {
		case workflow.TypeListString, workflow.TypeListRef:
			if _, err := coerceStringList(v); err != nil {
				return fmt.Errorf("invalid list value for %q: %w", k, err)
			}
		}
	}
	return nil
}

// builtInFrontmatterKeys lists keys handled by the taskFrontmatter struct.
var builtInFrontmatterKeys = map[string]bool{
	"id": true, "title": true, "type": true, "status": true,
	"tags": true, "dependsOn": true, "due": true, "recurrence": true,
	"assignee": true, "priority": true, "points": true,
}

// extractCustomFields reads custom field values from a raw frontmatter map.
// Built-in keys are skipped. Registered custom fields are coerced and returned
// in the first map. Unrecognised non-builtin keys are preserved verbatim in
// the second map so they survive a load→save round-trip.
//
// Phase 3: production loads go through buildFieldsFromFrontmatter in
// tiki_bridge.go — this helper survives only for the focused unit tests in
// store_test.go that exercise the registry/coercion rules in isolation.
func extractCustomFields(fmMap map[string]interface{}) (customFields, unknownFields map[string]interface{}, err error) {
	if fmMap == nil {
		return nil, nil, nil
	}

	registryChecked := false
	for key, raw := range fmMap {
		if builtInFrontmatterKeys[key] {
			continue
		}
		// defer registry check until we actually encounter a non-builtin key
		if !registryChecked {
			if err := config.RequireWorkflowRegistriesLoaded(); err != nil {
				return nil, nil, fmt.Errorf("extractCustomFields: %w", err)
			}
			registryChecked = true
		}
		fd, ok := workflow.Field(key)
		if ok && !fd.Custom {
			// registered built-in (e.g. synthetic read-only fields like
			// filepath). These are derived, never persisted — drop whatever
			// was in the file so stale values don't survive a round-trip.
			slog.Debug("dropping synthetic built-in field from frontmatter", "field", key)
			continue
		}
		if !ok {
			slog.Debug("preserving unknown field in frontmatter", "field", key)
			if unknownFields == nil {
				unknownFields = make(map[string]interface{})
			}
			unknownFields[key] = raw
			continue
		}
		val, err := coerceCustomValue(fd, raw)
		if err != nil {
			// stale value (e.g. removed enum option): demote to unknown so
			// the task still loads and the value survives for repair
			slog.Warn("demoting stale custom field value to unknown",
				"field", key, "error", err)
			if unknownFields == nil {
				unknownFields = make(map[string]interface{})
			}
			unknownFields[key] = raw
			continue
		}
		if customFields == nil {
			customFields = make(map[string]interface{})
		}
		customFields[key] = val
	}
	return customFields, unknownFields, nil
}

// coerceCustomValue converts a raw YAML value to the Go type expected by FieldDef.
func coerceCustomValue(fd workflow.FieldDef, raw interface{}) (interface{}, error) {
	switch fd.Type {
	case workflow.TypeString:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", raw)
		}
		return s, nil

	case workflow.TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for enum, got %T", raw)
		}
		for _, av := range fd.AllowedValues {
			if strings.EqualFold(s, av) {
				return av, nil // canonical casing
			}
		}
		return nil, fmt.Errorf("value %q not in allowed values %v", s, fd.AllowedValues)

	case workflow.TypeInt:
		switch v := raw.(type) {
		case int:
			return v, nil
		case float64:
			if v != float64(int(v)) {
				return nil, fmt.Errorf("value %g is not a whole number", v)
			}
			return int(v), nil
		default:
			return nil, fmt.Errorf("expected int, got %T", raw)
		}

	case workflow.TypeBool:
		b, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", raw)
		}
		return b, nil

	case workflow.TypeTimestamp:
		switch v := raw.(type) {
		case time.Time:
			return v, nil
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t, nil
			}
			if t, err := time.Parse("2006-01-02", v); err == nil {
				return t, nil
			}
			return nil, fmt.Errorf("cannot parse timestamp %q", v)
		default:
			return nil, fmt.Errorf("expected time or string for timestamp, got %T", raw)
		}

	case workflow.TypeListString:
		return coerceStringList(raw)

	case workflow.TypeListRef:
		ss, err := coerceStringList(raw)
		if err != nil {
			return nil, err
		}
		return collectionutil.NormalizeRefSet(ss), nil

	default:
		return raw, nil
	}
}

// coerceStringList converts a raw YAML value ([]interface{}) to []string.
func coerceStringList(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []interface{}:
		ss := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("list item: expected string, got %T", item)
			}
			ss = append(ss, s)
		}
		return collectionutil.NormalizeStringSet(ss), nil
	case []string:
		ss := make([]string, len(v))
		copy(ss, v)
		return collectionutil.NormalizeStringSet(ss), nil
	default:
		return nil, fmt.Errorf("expected list, got %T", raw)
	}
}

// Phase 3: the append{Custom,Unknown}Fields helpers are gone. Save now
// routes through marshalTikiFrontmatter in tiki_bridge.go, which emits
// workflow-schema keys, custom fields, and unknown fields through a
// single presence-driven path.

// saveTask is a Phase 5 compatibility adapter over saveTiki for callers
// (mostly tests) that still operate on *taskpkg.Task.
func (s *TikiStore) saveTask(task *taskpkg.Task) error {
	return s.saveTiki(taskToTiki(task))
}

// loadTaskFile is a Phase 5 compatibility adapter over loadTikiFile for
// callers (mostly tests) that expect a *taskpkg.Task back. The author and
// last-commit maps are not fetched here (tests pass nil and this helper
// is not used on the hot load path); callers that need git metadata use
// loadTikiFile directly.
func (s *TikiStore) loadTaskFile(path string) (*taskpkg.Task, error) {
	tk, err := s.loadTikiFile(path, nil, nil)
	if err != nil {
		return nil, err
	}
	pt := &parsedTiki{t: tk, raw: buildRawFromTiki(tk), stale: tk.StaleKeys()}
	return tikiToTask(pt), nil
}

// buildRawFromTiki reconstructs a raw frontmatter map from a Tiki's Fields
// for use in the IsWorkflowFrontmatter check inside tikiToTask. The raw map
// only needs to carry the keys that are present in Fields — the check is
// purely presence-based.
func buildRawFromTiki(tk *tiki.Tiki) map[string]interface{} {
	if tk == nil {
		return nil
	}
	raw := make(map[string]interface{}, len(tk.Fields)+2)
	raw["id"] = tk.ID
	if tk.Title != "" {
		raw["title"] = tk.Title
	}
	for k, v := range tk.Fields {
		raw[k] = v
	}
	return raw
}
