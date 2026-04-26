package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// identityOnlyStore is a minimal store.Store implementation that only
// answers GetCurrentUser. It exists so we can assert getCurrentUserName's
// projection contract without standing up a full in-memory store.
type identityOnlyStore struct {
	store.Store // embedded for default no-op-ish behavior
	name        string
	email       string
}

func (s *identityOnlyStore) GetCurrentUser() (string, string, error) {
	return s.name, s.email, nil
}

// expose a typed nil ReadStore implementation to avoid the embedded-nil panic
// paths in other tests — getCurrentUserName only touches GetCurrentUser.
var _ store.Store = (*identityOnlyStore)(nil)

// TestGetCurrentUserName_EmailOnly locks in the fix for the plugin-action
// regression the reviewer identified: plugin executors built via
// pluginBase.newExecutor used getCurrentUserName, which previously projected
// only `name`. With only identity.email configured that produced nil userFunc
// so plugin actions that called user() failed with "unavailable". The helper
// must now project name || email to match the runtime and trigger paths.
func TestGetCurrentUserName_EmailOnly(t *testing.T) {
	s := &identityOnlyStore{email: "me@example.com"}
	got := getCurrentUserName(s)
	if got != "me@example.com" {
		t.Errorf("got %q, want 'me@example.com' (email should be promoted when name is empty)", got)
	}
}

func TestGetCurrentUserName_NamePreferred(t *testing.T) {
	s := &identityOnlyStore{name: "Alice", email: "alice@example.com"}
	got := getCurrentUserName(s)
	if got != "Alice" {
		t.Errorf("got %q, want 'Alice'", got)
	}
}

func TestGetCurrentUserName_BothEmpty(t *testing.T) {
	s := &identityOnlyStore{}
	if got := getCurrentUserName(s); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestSetAuthorFromGit_EmailOnly_NoAngleEchoing asserts the raw-tuple
// contract at the controller layer: CreatedBy must not be formatted as
// `me@example.com <me@example.com>` when only email is configured. This
// mirrors the tikistore template test and covers the controller path that
// the UI uses for non-template task creation.
func TestSetAuthorFromGit_EmailOnly_NoAngleEchoing(t *testing.T) {
	s := &identityOnlyStore{email: "me@example.com"}
	tk := &task.Task{}
	setAuthorFromGit(tk, s)
	if tk.CreatedBy != "me@example.com" {
		t.Errorf("CreatedBy = %q, want 'me@example.com'", tk.CreatedBy)
	}
}
