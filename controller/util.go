package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// Helper functions shared across controllers.

// generateID creates a unique identifier
func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// setAuthorOnTiki best-effort populates createdBy on a tiki using the current git user via store.
func setAuthorOnTiki(tk *tikipkg.Tiki, taskStore store.Store) {
	if tk == nil {
		return
	}
	if existing, _, _ := tk.StringField("createdBy"); existing != "" {
		return
	}

	name, email, err := taskStore.GetCurrentUser()
	if err != nil {
		return
	}

	var author string
	switch {
	case name != "" && email != "":
		author = fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		author = name
	case email != "":
		author = email
	}
	if author != "" {
		tk.Set("createdBy", author)
	}
}

// getCurrentUserName returns the display string for the current Tiki identity
// (name, or email when name is empty). Delegates to store.CurrentUserDisplay
// so plugin-action executors see the same identity rule as the ruki runtime,
// CLI exec, triggers, and pipe-create setup.
func getCurrentUserName(taskStore store.Store) string {
	display, err := store.CurrentUserDisplay(taskStore)
	if err != nil {
		return ""
	}
	return display
}
