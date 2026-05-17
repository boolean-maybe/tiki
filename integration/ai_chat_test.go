package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/viper"
)

// TestTaskDetailView_ChatModifiesTask verifies the full chat action flow:
// key press → suspend → command modifies task file → resume → task reloaded.
func TestTaskDetailView_ChatModifiesTask(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// configure AI agent so the chat action is registered
	viper.Set("ai.agent", "claude")
	defer viper.Set("ai.agent", "")

	// create a task
	tikiID := "CHAT01"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Original Title", "ready", "story"); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// mock command runner: verifies claude args and modifies the task title on disk
	ta.NavController.SetCommandRunner(func(name string, args ...string) error {
		if name != "claude" {
			t.Errorf("expected command 'claude', got %q", name)
		}
		// verify --append-system-prompt is passed with task file path.
		// Phase 2: filenames are the bare uppercase id (no lowercase
		// convention), and the path comes from the task's own FilePath via
		// store.PathForID — which is an absolute path, not something the
		// test can reconstruct by joining ta.TikiDir + lower(id).
		if len(args) < 2 || args[0] != "--append-system-prompt" {
			t.Errorf("expected --append-system-prompt arg, got %v", args)
		} else if !strings.Contains(args[1], tikiID+".md") {
			t.Errorf("expected prompt to reference %s.md, got %q", tikiID, args[1])
		}
		tikiPath := filepath.Join(ta.TikiDir, tikiID+".md")
		content, err := os.ReadFile(tikiPath)
		if err != nil {
			return err
		}
		modified := strings.ReplaceAll(string(content), "Original Title", "AI Modified Title")
		return os.WriteFile(tikiPath, []byte(modified), 0644) //nolint:gosec // test file
	})

	// Phase 3: navigate to the configurable detail view (kind: detail)
	// directly, carrying the task id via PluginViewParams. The legacy
	// TikiDetailViewID is no longer the chat host.
	ta.NavController.PushView(
		model.DetailPluginViewID(),
		model.EncodePluginViewParams(model.PluginViewParams{TikiID: tikiID}),
	)
	ta.Draw()

	// press 'c' to invoke chat
	ta.SendKey(tcell.KeyRune, 'c', tcell.ModNone)

	// verify task was reloaded with the modified title
	updated := ta.TikiStore.GetTiki(tikiID)
	if updated == nil {
		t.Fatal("task not found after chat")
		return
	}
	if updated.Title != "AI Modified Title" {
		t.Errorf("title = %q, want %q", updated.Title, "AI Modified Title")
	}
}

// TestTaskDetailView_ChatNotAvailableWithoutConfig verifies the chat action
// is not triggered when ai.agent is not configured.
func TestTaskDetailView_ChatNotAvailableWithoutConfig(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// ensure ai.agent is empty
	viper.Set("ai.agent", "")

	// create a task
	tikiID := "NOCHAT"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Unchanged Title", "ready", "story"); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// mock command runner: should NOT be called
	called := false
	ta.NavController.SetCommandRunner(func(name string, args ...string) error {
		called = true
		return nil
	})

	// Phase 3: navigate to the configurable detail view (kind: detail)
	// directly, carrying the task id via PluginViewParams. The legacy
	// TikiDetailViewID is no longer the chat host.
	ta.NavController.PushView(
		model.DetailPluginViewID(),
		model.EncodePluginViewParams(model.PluginViewParams{TikiID: tikiID}),
	)
	ta.Draw()

	// press 'c' — should not trigger chat
	ta.SendKey(tcell.KeyRune, 'c', tcell.ModNone)

	if called {
		t.Error("command runner should not have been called when ai.agent is not configured")
	}

	// verify task is unchanged
	unchanged := ta.TikiStore.GetTiki(tikiID)
	if unchanged == nil {
		t.Fatal("task not found")
		return
	}
	if unchanged.Title != "Unchanged Title" {
		t.Errorf("title = %q, want %q", unchanged.Title, "Unchanged Title")
	}
}
