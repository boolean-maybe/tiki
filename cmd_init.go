package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitOpts holds parsed arguments for the init subcommand.
type InitOpts struct {
	Directory      string
	WorkflowName   string
	AISkills       []string
	Samples        bool
	NonInteractive bool
}

// parseInitArgs parses `tiki init` arguments using manual iteration
// matching the parseScopeArgs style.
func parseInitArgs(args []string) (InitOpts, error) {
	var opts InitOpts
	var directory string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--help" || arg == "-h":
			return InitOpts{}, errHelpRequested

		case arg == "-w" || arg == "--workflow":
			i++
			if i >= len(args) {
				return InitOpts{}, fmt.Errorf("%s requires a value", arg)
			}
			opts.WorkflowName = args[i] //nolint:gosec // G602: bounds checked above

		case strings.HasPrefix(arg, "--workflow="):
			opts.WorkflowName = strings.TrimPrefix(arg, "--workflow=")

		case arg == "--ai-skill":
			i++
			if i >= len(args) {
				return InitOpts{}, fmt.Errorf("--ai-skill requires a value")
			}
			opts.AISkills = splitAndTrim(args[i]) //nolint:gosec // G602: bounds checked above

		case strings.HasPrefix(arg, "--ai-skill="):
			opts.AISkills = splitAndTrim(strings.TrimPrefix(arg, "--ai-skill="))

		case arg == "--samples":
			opts.Samples = true

		case arg == "-n" || arg == "--non-interactive":
			opts.NonInteractive = true

		case strings.HasPrefix(arg, "-"):
			return InitOpts{}, fmt.Errorf("unknown flag: %s", arg)

		default:
			if directory != "" {
				return InitOpts{}, fmt.Errorf("multiple directories: %q and %q", directory, arg)
			}
			directory = arg
		}
	}

	if directory == "" {
		directory = "."
	}

	absDir, err := filepath.Abs(directory)
	if err != nil {
		return InitOpts{}, fmt.Errorf("resolve directory %q: %w", directory, err)
	}
	opts.Directory = absDir

	return opts, nil
}

// splitAndTrim splits a comma-separated string into trimmed non-empty parts.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ensureDirectoryAndGitRepo creates the directory if it doesn't exist and
// initializes a git repository if one isn't already present.
func ensureDirectoryAndGitRepo(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("directory %q: %w", dir, err)
		}
		//nolint:gosec // G301: 0755 is standard for project directories
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	if !tikistore.IsGitRepo(dir) {
		//nolint:gosec // G204: git init with user-provided directory (already validated)
		cmd := exec.Command("git", "init", dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init %q: %s", dir, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// validateInitOpts checks that the parsed options are valid before running init.
func validateInitOpts(opts InitOpts) error {
	if err := ensureDirectoryAndGitRepo(opts.Directory); err != nil {
		return err
	}

	for _, skill := range opts.AISkills {
		if _, ok := config.LookupAITool(skill); !ok {
			validKeys := make([]string, 0, len(config.AITools()))
			for _, t := range config.AITools() {
				validKeys = append(validKeys, t.Key)
			}
			return fmt.Errorf("unknown AI skill %q (valid: %s)", skill, strings.Join(validKeys, ", "))
		}
	}

	if opts.Samples && opts.WorkflowName != "" {
		return fmt.Errorf("--samples cannot be used with --workflow (samples are only valid for the bundled default workflow)")
	}

	if opts.WorkflowName != "" {
		if !config.ValidWorkflowName(opts.WorkflowName) {
			return fmt.Errorf("invalid workflow name %q: use letters, digits, hyphens, dots, or underscores", opts.WorkflowName)
		}
	}

	return nil
}

// runInit implements the `tiki init` subcommand. Returns an exit code.
func runInit(args []string) int {
	opts, err := parseInitArgs(args)
	if err != nil {
		if err == errHelpRequested {
			printInitUsage()
			return exitOK
		}
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printInitUsage()
		return exitUsage
	}

	if err := validateInitOpts(opts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if err := os.Chdir(opts.Directory); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: chdir to %s: %v\n", opts.Directory, err)
		return exitStartupFailure
	}

	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if config.IsProjectInitialized() {
		fmt.Println("project already initialized")
		return exitOK
	}

	// determine AI skills
	var aiSkills []string
	if opts.NonInteractive || len(opts.AISkills) > 0 {
		aiSkills = opts.AISkills
	} else {
		initOpts, proceed, promptErr := config.PromptForProjectInit(opts.WorkflowName != "")
		if promptErr != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", promptErr)
			return exitStartupFailure
		}
		if !proceed {
			return exitOK
		}
		aiSkills = initOpts.AITools
	}

	// determine sample creation
	createSamples := false
	if opts.WorkflowName == "" {
		if opts.NonInteractive {
			createSamples = opts.Samples
		} else {
			createSamples = true
		}
	}

	if err := config.BootstrapSystem(createSamples); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: bootstrap: %v\n", err)
		return exitStartupFailure
	}

	if len(aiSkills) > 0 {
		if err := config.InstallAISkills(aiSkills, tikiSkillMdContent, dokiSkillMdContent); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: some AI skills failed to install: %v\n", err)
		} else {
			fmt.Printf("installed AI skills for: %s\n", strings.Join(aiSkills, ", "))
		}
	}

	if err := config.InstallDefaultWorkflow(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: install default workflow: %v\n", err)
	}

	if opts.WorkflowName != "" {
		results, err := config.InstallWorkflow(opts.WorkflowName, config.ScopeLocal, config.DefaultWorkflowBaseURL)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: install workflow %q: %v\n", opts.WorkflowName, err)
			return exitStartupFailure
		}
		for _, r := range results {
			if r.Changed {
				fmt.Println("installed", r.Path)
			}
		}
	}

	fmt.Println("project initialized")
	return exitOK
}

func printInitUsage() {
	fmt.Print(`Usage: tiki init [directory] [options]

Initialize a tiki project. Creates the directory and git repo if needed.

Arguments:
  directory                    Target directory (default: current directory)

Options:
  -w, --workflow <name>        Install a named workflow (e.g. todo, kanban, bug-tracker)
  --ai-skill <list>            AI skills to install (comma-separated: claude,gemini)
  --samples                    Create bundled sample tasks (non-interactive only)
  -n, --non-interactive        Skip prompts, use flags/defaults only
  -h, --help                   Show this help message

Examples:
  tiki init                    Initialize current directory interactively
  tiki init my-project         Initialize my-project subdirectory
  tiki init -w todo            Initialize with the todo workflow
  tiki init -w kanban test1    Initialize test1 with the kanban workflow
  tiki init -n --samples       Initialize non-interactively with sample tasks
  tiki init --ai-skill claude  Initialize with Claude Code AI skill
`)
}
