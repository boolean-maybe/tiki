package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

// PromptForProjectInit presents a Huh form for project initialization.
// Returns (selectedAITools, proceed, error)
func PromptForProjectInit() ([]string, bool, error) {
	var selectedAITools []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Install AI skills for task automation? (optional)").
				Description("AI assistants can create, search, and manage tasks").
				Options(
					huh.NewOption("Claude Code (.claude/skills/)", "claude"),
					huh.NewOption("OpenAI Codex (.codex/skills/)", "codex"),
					huh.NewOption("OpenCode (.opencode/skill/)", "opencode"),
				).
				Value(&selectedAITools),
		),
	).WithTheme(huh.ThemeCharm())

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("form error: %w", err)
	}

	return selectedAITools, true, nil
}

// EnsureProjectInitialized bootstraps the project if .doc/tiki is missing.
// Returns (proceed, error).
// If proceed is false, the user canceled initialization.
func EnsureProjectInitialized(tikiSkillMdContent, dokiSkillMdContent string) (bool, error) {
	taskDir := GetTaskDir()
	if _, err := os.Stat(taskDir); err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("failed to stat task directory: %w", err)
		}

		selectedTools, proceed, err := PromptForProjectInit()
		if err != nil {
			return false, fmt.Errorf("failed to prompt for project initialization: %w", err)
		}
		if !proceed {
			return false, nil
		}

		if err := BootstrapSystem(); err != nil {
			return false, fmt.Errorf("failed to bootstrap project: %w", err)
		}

		// Install selected AI skills
		if len(selectedTools) > 0 {
			if err := installAISkills(selectedTools, tikiSkillMdContent, dokiSkillMdContent); err != nil {
				// Non-fatal - log warning but continue
				slog.Warn("some AI skills failed to install", "error", err)
				fmt.Println("You can manually copy ai/skills/tiki/SKILL.md and ai/skills/doki/SKILL.md to the appropriate directories.")
			} else {
				fmt.Printf("✓ Installed AI skills for: %s\n", strings.Join(selectedTools, ", "))
			}
		}

		return true, nil
	}

	return true, nil
}

// installAISkills writes the embedded SKILL.md content to selected AI tool directories.
// Returns an aggregated error if any installations fail, but continues attempting all.
func installAISkills(selectedTools []string, tikiSkillMdContent, dokiSkillMdContent string) error {
	if len(tikiSkillMdContent) == 0 {
		return fmt.Errorf("embedded tiki SKILL.md content is empty")
	}
	if len(dokiSkillMdContent) == 0 {
		return fmt.Errorf("embedded doki SKILL.md content is empty")
	}

	// Define target paths for both tiki and doki skills
	type skillPaths struct {
		tiki string
		doki string
	}

	toolPaths := map[string]skillPaths{
		"claude": {
			tiki: ".claude/skills/tiki/SKILL.md",
			doki: ".claude/skills/doki/SKILL.md",
		},
		"codex": {
			tiki: ".codex/skills/tiki/SKILL.md",
			doki: ".codex/skills/doki/SKILL.md",
		},
		"opencode": {
			tiki: ".opencode/skill/tiki/SKILL.md",
			doki: ".opencode/skill/doki/SKILL.md",
		},
	}

	var errs []error
	for _, tool := range selectedTools {
		paths, ok := toolPaths[tool]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown tool: %s", tool))
			continue
		}

		// Install tiki skill
		tikiDir := filepath.Dir(paths.tiki)
		//nolint:gosec // G301: 0755 is appropriate for user-owned skill directories
		if err := os.MkdirAll(tikiDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to create tiki directory for %s: %w", tool, err))
		} else if err := os.WriteFile(paths.tiki, []byte(tikiSkillMdContent), 0644); err != nil {
			errs = append(errs, fmt.Errorf("failed to write tiki SKILL.md for %s: %w", tool, err))
		} else {
			slog.Info("installed tiki AI skill", "tool", tool, "path", paths.tiki)
		}

		// Install doki skill
		dokiDir := filepath.Dir(paths.doki)
		//nolint:gosec // G301: 0755 is appropriate for user-owned skill directories
		if err := os.MkdirAll(dokiDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to create doki directory for %s: %w", tool, err))
		} else if err := os.WriteFile(paths.doki, []byte(dokiSkillMdContent), 0644); err != nil {
			errs = append(errs, fmt.Errorf("failed to write doki SKILL.md for %s: %w", tool, err))
		} else {
			slog.Info("installed doki AI skill", "tool", tool, "path", paths.doki)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
