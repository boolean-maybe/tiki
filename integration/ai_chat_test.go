package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
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
	taskID := "TIKI-CHAT01"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// mock command runner: verifies claude args and modifies the task title on disk
	ta.NavController.SetCommandRunner(func(name string, args ...string) error {
		if name != "claude" {
			t.Errorf("expected command 'claude', got %q", name)
		}
		// verify --append-system-prompt is passed with task file path
		if len(args) < 2 || args[0] != "--append-system-prompt" {
			t.Errorf("expected --append-system-prompt arg, got %v", args)
		} else if !strings.Contains(args[1], strings.ToLower(taskID)) {
			t.Errorf("expected prompt to reference task ID, got %q", args[1])
		}
		taskPath := filepath.Join(ta.TaskDir, strings.ToLower(taskID)+".md")
		content, err := os.ReadFile(taskPath)
		if err != nil {
			return err
		}
		modified := strings.ReplaceAll(string(content), "Original Title", "AI Modified Title")
		return os.WriteFile(taskPath, []byte(modified), 0644) //nolint:gosec // test file
	})

	// navigate to task detail
	ta.NavController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	ta.Draw()

	// press 'c' to invoke chat
	ta.SendKey(tcell.KeyRune, 'c', tcell.ModNone)

	// verify task was reloaded with the modified title
	updated := ta.TaskStore.GetTask(taskID)
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
	taskID := "TIKI-NOCHAT"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Unchanged Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// mock command runner: should NOT be called
	called := false
	ta.NavController.SetCommandRunner(func(name string, args ...string) error {
		called = true
		return nil
	})

	// navigate to task detail
	ta.NavController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	ta.Draw()

	// press 'c' — should not trigger chat
	ta.SendKey(tcell.KeyRune, 'c', tcell.ModNone)

	if called {
		t.Error("command runner should not have been called when ai.agent is not configured")
	}

	// verify task is unchanged
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatal("task not found")
		return
	}
	if task.Title != "Unchanged Title" {
		t.Errorf("title = %q, want %q", task.Title, "Unchanged Title")
	}
}
