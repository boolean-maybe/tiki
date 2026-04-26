package tikistore

import (
	"log/slog"
	"os"
	osuser "os/user"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/internal/git"
)

// identityResolver resolves the current Tiki identity from layered sources.
// Resolution order: configured identity → git (when gitUtil != nil) → OS user.
// Empty strings are returned when no source yields a value; callers should not
// treat this as an error.
type identityResolver struct {
	gitUtil git.GitOps
}

// newIdentityResolver builds a resolver backed by the given git utility.
// gitUtil may be nil, in which case the git layer is skipped.
func newIdentityResolver(gitUtil git.GitOps) *identityResolver {
	return &identityResolver{gitUtil: gitUtil}
}

// currentUser returns the raw name and email for the current Tiki identity.
// A git-layer error (e.g. `user.name` unset) is treated the same as an empty
// git result: we fall through to the OS layer instead of blocking `user()`.
// Errors are logged at debug level so repo misconfiguration is diagnosable
// without breaking the no-git workflow.
//
// The returned tuple is faithful to its source: if only `identity.email` is
// set, this returns ("", email, nil). Callers that want a single display
// string (e.g. ruki `user()`, the header "User" stat) go through the
// package-level helper store.CurrentUserDisplay, which owns the projection
// rule. Keeping the raw tuple here prevents attribution formatting like
// `me@example.com <me@example.com>` in CreatedBy.
func (r *identityResolver) currentUser() (name string, email string, err error) {
	if n, e := fromConfig(); n != "" || e != "" {
		return n, e, nil
	}
	if r.gitUtil != nil {
		gitName, gitEmail, gitErr := r.gitUtil.CurrentUser()
		if gitErr != nil {
			slog.Debug("git CurrentUser failed, falling back to OS user", "error", gitErr)
		} else if gitName != "" || gitEmail != "" {
			return gitName, gitEmail, nil
		}
	}
	if n := osUsername(); n != "" {
		return n, "", nil
	}
	return "", "", nil
}

// allUsers returns a deduplicated list of candidate assignee identities.
// In git mode, merges the configured identity with git's author list. If git
// AllUsers fails (e.g. no commits yet), we log and fall through so assignee
// selection still works in a fresh repo with only a configured identity.
// In no-git mode, returns the resolved identity (configured or OS).
func (r *identityResolver) allUsers() ([]string, error) {
	configured := configuredIdentityString()
	if r.gitUtil != nil {
		gitUsers, err := r.gitUtil.AllUsers()
		if err != nil {
			slog.Debug("git AllUsers failed, falling back to identity-only list", "error", err)
		} else if len(gitUsers) > 0 || configured != "" {
			return mergeUnique(configured, gitUsers), nil
		}
	}
	if configured != "" {
		return []string{configured}, nil
	}
	if osName := osUsername(); osName != "" {
		return []string{osName}, nil
	}
	return nil, nil
}

// fromConfig returns the configured identity from config/env, or empty strings.
func fromConfig() (name string, email string) {
	return config.GetIdentityName(), config.GetIdentityEmail()
}

// configuredIdentityString returns the single display string for the configured
// identity — name if set, otherwise email, otherwise empty. Shared by allUsers
// so "email only" configs still appear in the assignee picker.
func configuredIdentityString() string {
	name, email := fromConfig()
	if name != "" {
		return name
	}
	return email
}

// osUsername returns the OS account username via os/user, falling back to
// USER / LOGNAME / USERNAME env vars. Returns empty string when nothing is set.
// Deliberately does not invent a display name — predictability across machines
// matters more than aesthetics for no-git attribution.
func osUsername() string {
	if u, err := osuser.Current(); err == nil {
		if name := strings.TrimSpace(u.Username); name != "" {
			return name
		}
	}
	for _, key := range []string{"USER", "LOGNAME", "USERNAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// mergeUnique prepends head to tail when non-empty, preserving first-seen order
// and deduplicating case-sensitively (git author names are case-sensitive).
func mergeUnique(head string, tail []string) []string {
	seen := make(map[string]struct{}, len(tail)+1)
	result := make([]string, 0, len(tail)+1)
	if head != "" {
		seen[head] = struct{}{}
		result = append(result, head)
	}
	for _, name := range tail {
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
