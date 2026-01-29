package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// PromptForProjectInit presents a Huh form for project initialization.
// Returns (selectedAITools, proceed, error)
func PromptForProjectInit() ([]string, bool, error) {
	var selectedAITools []string

	// Create custom theme with brighter description and help text
	theme := huh.ThemeCharm()
	descriptionColor := lipgloss.Color("189") // Light purple/blue for description
	helpKeyColor := lipgloss.Color("117")     // Light blue for keys
	helpDescColor := lipgloss.Color("252")    // Bright gray for descriptions
	theme.Focused.Description = lipgloss.NewStyle().Foreground(descriptionColor)
	theme.Blurred.Description = lipgloss.NewStyle().Foreground(descriptionColor)
	theme.Help.ShortKey = lipgloss.NewStyle().Foreground(helpKeyColor).Bold(true)
	theme.Help.ShortDesc = lipgloss.NewStyle().Foreground(helpDescColor)
	theme.Help.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	theme.Help.FullKey = lipgloss.NewStyle().Foreground(helpKeyColor).Bold(true)
	theme.Help.FullDesc = lipgloss.NewStyle().Foreground(helpDescColor)
	theme.Help.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Create custom keymap with Esc bound to quit
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "quit"),
	)

	description := `
This will initialize your project by creating directories and sample tiki files:

- .doc/doki directory to hold your Markdown documents
- .doc/tiki directory to hold your tasks
- a sample tiki task and a document

Additionally, optional AI skills are installed if you choose to
AI skills extend your AI assistant with commands to manage tasks and documentation:

• 'tiki' skill - Create, view, update, delete task tickets (.doc/tiki/*.md)
• 'doki' skill - Create and manage documentation files (.doc/doki/*.md)

Select AI assistants to install (optional), then press Enter to continue.
Press Esc to cancel project initialization.`

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Initialize project").
				Description(description).
				Options(
					huh.NewOption("Claude Code (.claude/skills/)", "claude"),
					huh.NewOption("OpenAI Codex (.codex/skills/)", "codex"),
					huh.NewOption("OpenCode (.opencode/skill/)", "opencode"),
				).
				Filterable(false).
				Value(&selectedAITools),
		),
	).WithTheme(theme).
		WithKeyMap(keymap).
		WithProgramOptions(tea.WithAltScreen())

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
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
