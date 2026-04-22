package config

import (
	"encoding/json"
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

// IsProjectInitialized returns true if the project has been initialized
// (i.e., the .doc/tiki directory exists).
func IsProjectInitialized() bool {
	taskDir := GetTaskDir()
	info, err := os.Stat(taskDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// InitOptions holds user choices from the init dialog.
type InitOptions struct {
	AITools []string
}

// PromptForProjectInit presents a Huh form for project initialization.
// hasCustomWorkflow adjusts the description text to reflect whether bundled
// samples will be created. Returns (options, proceed, error).
func PromptForProjectInit(hasCustomWorkflow bool) (InitOptions, bool, error) {
	var opts InitOptions

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

	var description string
	if hasCustomWorkflow {
		description = `
This will initialize your project with a custom workflow:

- .doc/doki directory to hold your Markdown documents
- .doc/tiki directory to hold your tasks
- custom workflow installed as .doc/workflow.yaml

No sample tasks will be created with a custom workflow.

Additionally, optional AI skills are installed if you choose to.
AI skills extend your AI assistant with commands to manage tasks and documentation:

• 'tiki' skill - Create, view, update, delete task tickets (.doc/tiki/*.md)
• 'doki' skill - Create and manage documentation files (.doc/doki/*.md)

Select AI assistants to install (optional), then press Enter to continue.
Press Esc to cancel project initialization.`
	} else {
		description = `
This will initialize your project by creating directories and bundled sample tikis:

- .doc/doki directory to hold your Markdown documents
- .doc/tiki directory to hold your tasks
- bundled sample tikis and a document

Additionally, optional AI skills are installed if you choose to.
AI skills extend your AI assistant with commands to manage tasks and documentation:

• 'tiki' skill - Create, view, update, delete task tickets (.doc/tiki/*.md)
• 'doki' skill - Create and manage documentation files (.doc/doki/*.md)

Select AI assistants to install (optional), then press Enter to continue.
Press Esc to cancel project initialization.`
	}

	aiOptions := make([]huh.Option[string], 0, len(AITools()))
	for _, t := range AITools() {
		aiOptions = append(aiOptions, huh.NewOption(fmt.Sprintf("%s (%s/)", t.DisplayName, t.SkillDir), t.Key))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Initialize project").
				Description(description).
				Options(aiOptions...).
				Filterable(false).
				Value(&opts.AITools),
		),
	).WithTheme(theme).
		WithKeyMap(keymap).
		WithProgramOptions(tea.WithAltScreen())

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return InitOptions{}, false, nil
		}
		return InitOptions{}, false, fmt.Errorf("form error: %w", err)
	}

	return opts, true, nil
}

// EnsureProjectInitialized bootstraps the project if .doc/tiki is missing.
// Returns (proceed, error).
// If proceed is false, the user canceled initialization.
// gitAdd, when non-nil, is called to stage created files.
func EnsureProjectInitialized(tikiSkillMdContent, dokiSkillMdContent string, gitAdd func(...string) error) (bool, error) {
	taskDir := GetTaskDir()
	if _, err := os.Stat(taskDir); err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("failed to stat task directory: %w", err)
		}

		opts, proceed, err := PromptForProjectInit(false)
		if err != nil {
			return false, fmt.Errorf("failed to prompt for project initialization: %w", err)
		}
		if !proceed {
			return false, nil
		}

		if err := BootstrapSystem(true, gitAdd); err != nil {
			return false, fmt.Errorf("failed to bootstrap project: %w", err)
		}

		// Install selected AI skills
		if len(opts.AITools) > 0 {
			if err := InstallAISkills(opts.AITools, tikiSkillMdContent, dokiSkillMdContent); err != nil {
				// Non-fatal - log warning but continue
				slog.Warn("some AI skills failed to install", "error", err)
				fmt.Println("You can manually copy ai/skills/tiki/SKILL.md and ai/skills/doki/SKILL.md to the appropriate directories.")
			} else {
				fmt.Printf("✓ Installed AI skills for: %s\n", strings.Join(opts.AITools, ", "))
			}
		}

		return true, nil
	}

	return true, nil
}

// InstallAISkills writes the embedded SKILL.md content to selected AI tool directories.
// Returns an aggregated error if any installations fail, but continues attempting all.
func InstallAISkills(selectedTools []string, tikiSkillMdContent, dokiSkillMdContent string) error {
	if len(tikiSkillMdContent) == 0 {
		return fmt.Errorf("embedded tiki SKILL.md content is empty")
	}
	if len(dokiSkillMdContent) == 0 {
		return fmt.Errorf("embedded doki SKILL.md content is empty")
	}

	type skillDef struct {
		name    string
		content string
	}
	skills := []skillDef{
		{"tiki", tikiSkillMdContent},
		{"doki", dokiSkillMdContent},
	}

	skillNames := make([]string, len(skills))
	for i, s := range skills {
		skillNames[i] = s.name
	}

	var errs []error
	for _, toolKey := range selectedTools {
		tool, ok := LookupAITool(toolKey)
		if !ok {
			errs = append(errs, fmt.Errorf("unknown tool: %s", toolKey))
			continue
		}

		for _, skill := range skills {
			path := tool.SkillPath(skill.name)
			dir := filepath.Dir(path)
			//nolint:gosec // G301: 0755 is appropriate for user-owned skill directories
			if err := os.MkdirAll(dir, 0755); err != nil {
				errs = append(errs, fmt.Errorf("failed to create %s directory for %s: %w", skill.name, toolKey, err))
			} else if err := os.WriteFile(path, []byte(skill.content), 0644); err != nil {
				errs = append(errs, fmt.Errorf("failed to write %s SKILL.md for %s: %w", skill.name, toolKey, err))
			} else {
				slog.Info("installed AI skill", "tool", toolKey, "skill", skill.name, "path", path)
			}
		}

		if tool.SettingsFile != "" {
			if err := ensureSkillPermissions(tool.SettingsFile, skillNames); err != nil {
				errs = append(errs, fmt.Errorf("failed to update %s settings: %w", toolKey, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func skillPermissionEntry(name string) string {
	return fmt.Sprintf("Skill(%s)", name)
}

// ensureSkillPermissions creates or updates a settings file to include
// Skill(<name>) entries in permissions.allow for each given skill name.
// Existing permissions and other top-level keys are preserved.
func ensureSkillPermissions(settingsPath string, skillNames []string) error {
	settings := make(map[string]any)

	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", settingsPath, err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, err)
		}
	}

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = make(map[string]any)
		settings["permissions"] = perms
	}

	allowRaw, _ := perms["allow"].([]any)
	existing := make(map[string]bool, len(allowRaw))
	for _, v := range allowRaw {
		if s, ok := v.(string); ok {
			existing[s] = true
		}
	}

	changed := false
	for _, name := range skillNames {
		entry := skillPermissionEntry(name)
		if !existing[entry] {
			allowRaw = append(allowRaw, entry)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	perms["allow"] = allowRaw
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0750); err != nil {
		return fmt.Errorf("creating directory for %s: %w", settingsPath, err)
	}
	//nolint:gosec // G306: 0644 is appropriate for user settings files
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	slog.Info("updated Claude settings", "path", settingsPath, "skills", skillNames)
	return nil
}
