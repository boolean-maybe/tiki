package tikistore

import (
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/internal/git"
)

// fakeGitOps is a minimal GitOps implementation for identity resolver tests.
// Only methods exercised by the resolver have meaningful behavior; the rest
// return zero values so accidental usage surfaces as empty data, not panics.
type fakeGitOps struct {
	name      string
	email     string
	userErr   error
	users     []string
	usersErr  error
	branchVal string
	branchErr error
}

func (f *fakeGitOps) Add(_ ...string) error                    { return nil }
func (f *fakeGitOps) Remove(_ ...string) error                 { return nil }
func (f *fakeGitOps) CurrentUser() (string, string, error)     { return f.name, f.email, f.userErr }
func (f *fakeGitOps) Author(_ string) (*git.AuthorInfo, error) { return nil, nil }
func (f *fakeGitOps) AllAuthors(_ string) (map[string]*git.AuthorInfo, error) {
	return nil, nil
}
func (f *fakeGitOps) LastCommitTime(_ string) (time.Time, error) {
	return time.Time{}, nil
}
func (f *fakeGitOps) AllLastCommitTimes(_ string) (map[string]time.Time, error) {
	return nil, nil
}
func (f *fakeGitOps) CurrentBranch() (string, error) { return f.branchVal, f.branchErr }
func (f *fakeGitOps) FileVersionsSince(_ string, _ time.Time, _ bool) ([]git.FileVersion, error) {
	return nil, nil
}
func (f *fakeGitOps) AllFileVersionsSince(_ string, _ time.Time, _ bool) (map[string][]git.FileVersion, error) {
	return nil, nil
}
func (f *fakeGitOps) AllUsers() ([]string, error) { return f.users, f.usersErr }

// isolateConfig puts the test in a clean config sandbox:
//   - cwd is moved to a fresh temp dir (so `./config.yaml` cannot be found)
//   - XDG_CONFIG_HOME is pointed at that temp dir (so the developer's real
//     `~/.config/tiki/config.yaml` cannot leak into the test)
//   - PathManager is reset so lookups re-read the new paths
//
// Without this, a stale `appConfig` from a parent test or a real user config
// on the dev machine can silently corrupt assertions about "unset identity".
func isolateConfig(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// change cwd so `./config.yaml` is also absent; restore on cleanup
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)
}

// setIdentityEnv loads config after setting identity env vars. Requires
// isolateConfig to have run first; otherwise real project config may win.
func setIdentityEnv(t *testing.T, name, email string) {
	t.Helper()
	t.Setenv("TIKI_IDENTITY_NAME", name)
	t.Setenv("TIKI_IDENTITY_EMAIL", email)
	if _, err := config.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
}

// resetConfigIdentity isolates config and explicitly clears identity env vars.
func resetConfigIdentity(t *testing.T) {
	t.Helper()
	isolateConfig(t)
	setIdentityEnv(t, "", "")
}

// resetConfigIdentityWithName isolates config and sets just the identity name.
// Email-setting variants inline the calls since no current test needs a name
// + email combination with clean isolation.
func resetConfigIdentityWithName(t *testing.T, name string) {
	t.Helper()
	isolateConfig(t)
	setIdentityEnv(t, name, "")
}

func TestIdentityResolver_ConfigTakesPrecedence(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice Config", "alice@example.com")

	git := &fakeGitOps{name: "Git User", email: "git@example.com"}
	r := newIdentityResolver(git)

	name, email, err := r.currentUser()
	if err != nil {
		t.Fatalf("currentUser: %v", err)
	}
	if name != "Alice Config" {
		t.Errorf("name = %q, want 'Alice Config'", name)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want 'alice@example.com'", email)
	}
}

// TestIdentityResolver_EmailOnlyConfig_CurrentUser_KeepsRawTuple asserts the
// resolver's raw contract: currentUser returns the configured fields verbatim,
// without promoting email into name. This matters for attribution (CreatedBy)
// which would otherwise format as `me@example.com <me@example.com>`. The
// display projection lives separately in store.CurrentUserDisplay.
func TestIdentityResolver_EmailOnlyConfig_CurrentUser_KeepsRawTuple(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "", "me@example.com")

	r := newIdentityResolver(nil)
	name, email, err := r.currentUser()
	if err != nil {
		t.Fatalf("currentUser: %v", err)
	}
	if name != "" {
		t.Errorf("name = %q, want empty (no promotion in raw tuple)", name)
	}
	if email != "me@example.com" {
		t.Errorf("email = %q, want 'me@example.com'", email)
	}
}

func TestIdentityResolver_EmailOnlyConfig_AllUsersIncludesEmail(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "", "me@example.com")

	r := newIdentityResolver(nil)
	users, err := r.allUsers()
	if err != nil {
		t.Fatalf("allUsers: %v", err)
	}
	if !reflect.DeepEqual(users, []string{"me@example.com"}) {
		t.Errorf("users = %v, want [me@example.com]", users)
	}
}

func TestIdentityResolver_GitFallback(t *testing.T) {
	resetConfigIdentity(t)

	git := &fakeGitOps{name: "Git User", email: "git@example.com"}
	r := newIdentityResolver(git)

	name, email, err := r.currentUser()
	if err != nil {
		t.Fatalf("currentUser: %v", err)
	}
	if name != "Git User" {
		t.Errorf("name = %q, want 'Git User'", name)
	}
	if email != "git@example.com" {
		t.Errorf("email = %q, want 'git@example.com'", email)
	}
}

func TestIdentityResolver_OSFallbackWhenNoGit(t *testing.T) {
	resetConfigIdentity(t)
	t.Setenv("USER", "os-account")

	r := newIdentityResolver(nil)
	name, email, err := r.currentUser()
	if err != nil {
		t.Fatalf("currentUser: %v", err)
	}
	if email != "" {
		t.Errorf("email = %q, want empty (OS fallback does not invent email)", email)
	}
	if name == "" {
		t.Error("expected non-empty OS-derived name, got empty")
	}
}

func TestIdentityResolver_OSFallbackEnvPath(t *testing.T) {
	resetConfigIdentity(t)
	// os/user.Current() nearly always succeeds on dev/CI hosts, so we cannot
	// reliably force the env-var branch. Just assert osUsername does not panic.
	got := osUsername()
	_ = got
}

func TestIdentityResolver_GitReturnsEmptyFallsThroughToOS(t *testing.T) {
	resetConfigIdentity(t)
	t.Setenv("USER", "os-account")

	git := &fakeGitOps{} // empty name/email, no error
	r := newIdentityResolver(git)

	name, _, err := r.currentUser()
	if err != nil {
		t.Fatalf("currentUser: %v", err)
	}
	if name == "" {
		t.Error("expected OS fallback when git returns empty, got empty")
	}
}

func TestIdentityResolver_GitErrorFallsThroughToOS(t *testing.T) {
	resetConfigIdentity(t)
	t.Setenv("USER", "os-account")

	gitErr := errors.New("git user not configured")
	git := &fakeGitOps{userErr: gitErr}
	r := newIdentityResolver(git)

	name, _, err := r.currentUser()
	// git errors must NOT block user() — otherwise a repo without user.name
	// can never resolve even a configured identity or OS fallback.
	if err != nil {
		t.Errorf("err = %v, want nil (should fall through on git error)", err)
	}
	if name == "" {
		t.Error("expected OS fallback name when git errors, got empty")
	}
}

func TestIdentityResolver_GitErrorButConfigSetWinsConfig(t *testing.T) {
	resetConfigIdentityWithName(t, "Config Wins")

	git := &fakeGitOps{userErr: errors.New("no user configured")}
	r := newIdentityResolver(git)

	name, _, err := r.currentUser()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if name != "Config Wins" {
		t.Errorf("name = %q, want 'Config Wins'", name)
	}
}

func TestIdentityResolver_AllUsers_GitErrorFallsThroughToConfig(t *testing.T) {
	resetConfigIdentityWithName(t, "Only Config")

	git := &fakeGitOps{usersErr: errors.New("no commits yet")}
	r := newIdentityResolver(git)

	users, err := r.allUsers()
	if err != nil {
		t.Errorf("err = %v, want nil (should fall through on git error)", err)
	}
	if !reflect.DeepEqual(users, []string{"Only Config"}) {
		t.Errorf("users = %v, want [Only Config]", users)
	}
}

func TestIdentityResolver_AllUsers_GitMergesWithConfig(t *testing.T) {
	resetConfigIdentityWithName(t, "Alice Config")

	git := &fakeGitOps{users: []string{"Git Alice", "Git Bob"}}
	r := newIdentityResolver(git)

	users, err := r.allUsers()
	if err != nil {
		t.Fatalf("allUsers: %v", err)
	}
	want := []string{"Alice Config", "Git Alice", "Git Bob"}
	if !reflect.DeepEqual(users, want) {
		t.Errorf("users = %v, want %v", users, want)
	}
}

func TestIdentityResolver_AllUsers_NoGitReturnsConfiguredOrOS(t *testing.T) {
	resetConfigIdentity(t)
	t.Setenv("USER", "os-account")

	r := newIdentityResolver(nil)
	users, err := r.allUsers()
	if err != nil {
		t.Fatalf("allUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("users length = %d, want 1", len(users))
	}
	if users[0] == "" {
		t.Errorf("expected non-empty user in no-git mode, got empty")
	}
}

func TestIdentityResolver_AllUsers_NoGitWithConfig(t *testing.T) {
	resetConfigIdentityWithName(t, "Alice")

	r := newIdentityResolver(nil)
	users, err := r.allUsers()
	if err != nil {
		t.Fatalf("allUsers: %v", err)
	}
	if !reflect.DeepEqual(users, []string{"Alice"}) {
		t.Errorf("users = %v, want [Alice]", users)
	}
}

func TestTikiStore_GetCurrentUser_NoGitUsesConfig(t *testing.T) {
	isolateConfig(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	setIdentityEnv(t, "Configured Alice", "alice@example.com")

	tmpDir := t.TempDir()
	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	name, email, err := s.GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if name != "Configured Alice" {
		t.Errorf("name = %q, want 'Configured Alice'", name)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want 'alice@example.com'", email)
	}
}

func TestTikiStore_GetStats_UsesConfiguredIdentity(t *testing.T) {
	isolateConfig(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	setIdentityEnv(t, "Header Alice", "")

	tmpDir := t.TempDir()
	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	stats := s.GetStats()
	var userStat string
	for _, stat := range stats {
		if stat.Name == "User" {
			userStat = stat.Value
		}
	}
	if userStat != "Header Alice" {
		t.Errorf("User stat = %q, want 'Header Alice'", userStat)
	}
}

func TestTikiStore_GetStats_EmailOnly_ProjectsEmail(t *testing.T) {
	isolateConfig(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	setIdentityEnv(t, "", "me@example.com")

	tmpDir := t.TempDir()
	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	stats := s.GetStats()
	var userStat string
	for _, stat := range stats {
		if stat.Name == "User" {
			userStat = stat.Value
		}
	}
	if userStat != "me@example.com" {
		t.Errorf("User stat = %q, want 'me@example.com' (email-only config should project email)", userStat)
	}
}

// TestTikiStore_GetStats_MatchesSharedHelper asserts that the header "User"
// stat agrees with store.CurrentUserDisplay for every (name, email) shape.
// If GetStats ever drifts from the shared helper — the path every other
// caller (ruki, plugin actions, triggers, pipe-create) now funnels through —
// this test fails loudly.
func TestTikiStore_GetStats_MatchesSharedHelper(t *testing.T) {
	cases := []struct {
		label string
		name  string
		email string
	}{
		{"name and email", "Alice", "alice@example.com"},
		{"name only", "Alice", ""},
		{"email only", "", "me@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			isolateConfig(t)
			t.Setenv("TIKI_STORE_GIT", "false")
			setIdentityEnv(t, tc.name, tc.email)

			tmpDir := t.TempDir()
			s, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore: %v", err)
			}

			want, err := store.CurrentUserDisplay(s)
			if err != nil {
				t.Fatalf("CurrentUserDisplay: %v", err)
			}

			stats := s.GetStats()
			var got string
			for _, stat := range stats {
				if stat.Name == "User" {
					got = stat.Value
				}
			}
			if got != want {
				t.Errorf("User stat = %q, want %q (GetStats must delegate to store.CurrentUserDisplay)", got, want)
			}
		})
	}
}

func TestTikiStore_GetAllUsers_NoGitReturnsConfigured(t *testing.T) {
	isolateConfig(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	setIdentityEnv(t, "Allusers Alice", "")

	tmpDir := t.TempDir()
	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	users, err := s.GetAllUsers()
	if err != nil {
		t.Fatalf("GetAllUsers: %v", err)
	}
	if !reflect.DeepEqual(users, []string{"Allusers Alice"}) {
		t.Errorf("users = %v, want [Allusers Alice]", users)
	}
}

func TestMergeUnique(t *testing.T) {
	tests := []struct {
		name string
		head string
		tail []string
		want []string
	}{
		{"head prepended", "Alice", []string{"Bob", "Carol"}, []string{"Alice", "Bob", "Carol"}},
		{"head duplicates git user", "Alice", []string{"Alice", "Bob"}, []string{"Alice", "Bob"}},
		{"empty head", "", []string{"Bob", "Carol"}, []string{"Bob", "Carol"}},
		{"empty tail", "Alice", nil, []string{"Alice"}},
		{"empty entries skipped", "Alice", []string{"", "Bob", ""}, []string{"Alice", "Bob"}},
		{"all empty", "", nil, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeUnique(tt.head, tt.tail)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeUnique(%q, %v) = %v, want %v", tt.head, tt.tail, got, tt.want)
			}
		})
	}
}

func TestOSUsername_ReturnsSomething(t *testing.T) {
	if _, err := os.Hostname(); err != nil {
		t.Skip("environment has no hostname; skipping OS user check")
	}
	_ = osUsername()
}
