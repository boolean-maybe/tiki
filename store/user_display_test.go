package store

import (
	"errors"
	"testing"

	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// fakeUserStore is a minimal ReadStore that only implements the methods
// CurrentUserDisplay touches. Every other method returns a zero value.
type fakeUserStore struct {
	name    string
	email   string
	userErr error
}

func (f *fakeUserStore) GetTask(string) *task.Task               { return nil }
func (f *fakeUserStore) GetAllTasks() []*task.Task               { return nil }
func (f *fakeUserStore) GetTiki(string) *tikipkg.Tiki            { return nil }
func (f *fakeUserStore) GetAllTikis() []*tikipkg.Tiki            { return nil }
func (f *fakeUserStore) NewTikiTemplate() (*tikipkg.Tiki, error) { return nil, nil }
func (f *fakeUserStore) Search(string, func(*task.Task) bool) []task.SearchResult {
	return nil
}
func (f *fakeUserStore) SearchTikis(string, func(*tikipkg.Tiki) bool) []*tikipkg.Tiki {
	return nil
}
func (f *fakeUserStore) GetCurrentUser() (string, string, error) {
	return f.name, f.email, f.userErr
}
func (f *fakeUserStore) GetStats() []Stat                     { return nil }
func (f *fakeUserStore) GetAllUsers() ([]string, error)       { return nil, nil }
func (f *fakeUserStore) NewTaskTemplate() (*task.Task, error) { return nil, nil }
func (f *fakeUserStore) AddListener(ChangeListener) int       { return 0 }
func (f *fakeUserStore) RemoveListener(int)                   {}
func (f *fakeUserStore) Reload() error                        { return nil }
func (f *fakeUserStore) ReloadTask(string) error              { return nil }
func (f *fakeUserStore) PathForID(string) string              { return "" }

func TestCurrentUserDisplay_NamePreferred(t *testing.T) {
	s := &fakeUserStore{name: "Alice", email: "alice@example.com"}
	got, err := CurrentUserDisplay(s)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "Alice" {
		t.Errorf("got %q, want 'Alice'", got)
	}
}

// TestCurrentUserDisplay_EmailPromotedWhenNameEmpty locks in the fix for the
// projection regression: callers that only look at `name` silently lost
// email-only identities. The shared helper must project name || email so
// every consumer (plugin actions, triggers, cli exec, header stat) agrees.
func TestCurrentUserDisplay_EmailPromotedWhenNameEmpty(t *testing.T) {
	s := &fakeUserStore{name: "", email: "me@example.com"}
	got, err := CurrentUserDisplay(s)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "me@example.com" {
		t.Errorf("got %q, want 'me@example.com' (email must be promoted when name is empty)", got)
	}
}

func TestCurrentUserDisplay_BothEmptyReturnsEmpty(t *testing.T) {
	s := &fakeUserStore{}
	got, err := CurrentUserDisplay(s)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestCurrentUserDisplay_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	s := &fakeUserStore{userErr: wantErr}
	_, err := CurrentUserDisplay(s)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

// TestCurrentUserDisplayFunc_ReturnsNilWhenEmpty asserts the closure variant's
// contract — triggers and pipe setup call this form to get a `func() string`
// or nil so the executor can produce a clean "unavailable" error for user().
func TestCurrentUserDisplayFunc_ReturnsNilWhenEmpty(t *testing.T) {
	s := &fakeUserStore{}
	fn, err := CurrentUserDisplayFunc(s)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if fn != nil {
		t.Errorf("fn = non-nil, want nil when no identity resolves")
	}
}

func TestCurrentUserDisplayFunc_ClosesOverEmailOnly(t *testing.T) {
	s := &fakeUserStore{email: "me@example.com"}
	fn, err := CurrentUserDisplayFunc(s)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if fn == nil {
		t.Fatal("fn = nil, want closure projecting email")
	}
	if got := fn(); got != "me@example.com" {
		t.Errorf("fn() = %q, want 'me@example.com'", got)
	}
}
