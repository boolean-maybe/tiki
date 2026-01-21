package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// Helper functions shared across controllers.

// generateID creates a unique identifier
func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// setAuthorFromGit best-effort populates CreatedBy using current git user via store.
func setAuthorFromGit(task *task.Task, taskStore store.Store) {
	if task == nil || task.CreatedBy != "" {
		return
	}

	name, email, err := taskStore.GetCurrentUser()
	if err != nil {
		return
	}

	switch {
	case name != "" && email != "":
		task.CreatedBy = fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		task.CreatedBy = name
	case email != "":
		task.CreatedBy = email
	}
}

func getCurrentUserName(taskStore store.Store) string {
	name, _, err := taskStore.GetCurrentUser()
	if err != nil {
		return ""
	}
	return name
}
