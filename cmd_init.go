package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitOpts holds parsed arguments for the init subcommand.
type InitOpts struct {
	Directory       string
	WorkflowName    string
	WorkflowSource  config.WorkflowSource
	WorkflowContent string
	AISkills        []string
	Samples         bool
	NonInteractive  bool
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

// ensureDirectory creates the directory if it doesn't exist and verifies it is
// a directory if it does. No git side effects — safe to call before config load
// and before os.Chdir.
func ensureDirectory(dir string) error {
	info, err := os.Stat(dir) //nolint:gosec // G703: dir comes from filepath.Abs in parseInitArgs
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("directory %q: %w", dir, err)
		}
		//nolint:gosec // G301: 0755 is standard for project directories
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}
	return nil
}

// reconcileInitGit brings the current directory's git state in line with the
// resolved `store.git` config. Must be called after LoadConfig so GetStoreGit
// is authoritative. Idempotent: safe to call on fresh dirs, existing dirs, and
// pre-existing git repos.
func reconcileInitGit() error {
	if !config.GetStoreGit() {
		// user disabled git-backed store — leave any existing .git/ alone.
		_, _ = fmt.Fprintln(os.Stderr, "info: store.git=false; skipping git repo init")
		return nil
	}

	// check for a local .git entry, not an ancestor repo. tikistore.IsGitRepo
	// walks up the directory tree, so running `tiki init ./sub` from inside an
	// existing checkout would otherwise attach to the parent repo. os.Stat
	// answers the right question: does *this* directory own a repo?
	if _, err := os.Stat(".git"); err == nil {
		return nil
	}

	if err := tikistore.GitInit("."); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

// validateInitOpts checks that the parsed options are valid before running init.
// It classifies the workflow input and stores the resolved WorkflowSource on opts
// so that file paths survive the later os.Chdir.
// Workflow validation runs before directory creation to avoid side effects on failure.
func validateInitOpts(opts *InitOpts) error {
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
		src, err := config.ClassifyWorkflowInput(opts.WorkflowName)
		if err != nil {
			return fmt.Errorf("invalid workflow source %q: %w", opts.WorkflowName, err)
		}
		switch src.Kind {
		case config.WorkflowSourceEmbedded:
			if _, ok := config.LookupEmbeddedWorkflow(src.Name); !ok {
				return fmt.Errorf("unknown workflow %q (available: %s)",
					src.Name, strings.Join(config.EmbeddedWorkflowNames(), ", "))
			}
		case config.WorkflowSourceFile:
			info, err := os.Stat(src.Name)
			if err != nil {
				return fmt.Errorf("workflow file %q: %w", src.Name, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("workflow path %q is not a regular file", src.Name)
			}
		case config.WorkflowSourceURL:
			// validated below via fetch
		}

		// pre-fetch and validate content so failures happen before project bootstrap
		content, err := config.FetchWorkflowContent(src)
		if err != nil {
			return fmt.Errorf("fetch workflow %q: %w", opts.WorkflowName, err)
		}
		if src.Kind != config.WorkflowSourceEmbedded {
			vw, err := config.ValidateWorkflowContent(content)
			if err != nil {
				return fmt.Errorf("invalid workflow %q: %w", opts.WorkflowName, err)
			}
			if err := validateWorkflowViews(vw, content); err != nil {
				return fmt.Errorf("invalid workflow %q: %w", opts.WorkflowName, err)
			}
		}

		opts.WorkflowSource = src
		opts.WorkflowContent = content
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

	if err := validateInitOpts(&opts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if err := ensureDirectory(opts.Directory); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if err := os.Chdir(opts.Directory); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: chdir to %s: %v\n", opts.Directory, err)
		return exitStartupFailure
	}

	// reset path manager so InitPaths observes the new cwd, then load config
	// so GetStoreGit reflects TIKI_STORE_GIT / config.yaml values before
	// reconcileInitGit runs. Matches cmd_demo.go:52-59.
	config.ResetPathManager()
	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if _, err := bootstrap.LoadConfig(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if err := reconcileInitGit(); err != nil {
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

	var gitAdd func(...string) error
	if config.GetStoreGit() {
		gitAdd = tikistore.NewGitAdder("")
	}
	if err := config.BootstrapSystem(createSamples, gitAdd); err != nil {
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
		results, err := config.InstallWorkflowFromContent(opts.WorkflowContent, config.ScopeLocal)
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

Initialize a tiki project. Creates the directory, and initializes a git repo
if store.git is enabled (the default; see config.md).

Arguments:
  directory                    Target directory (default: current directory)

Options:
  -w, --workflow <source>      Install a workflow (embedded name, file path, or URL)
  --ai-skill <list>            AI skills to install (comma-separated: claude,gemini)
  --samples                    Create bundled sample tasks (non-interactive only)
  -n, --non-interactive        Skip prompts, use flags/defaults only
  -h, --help                   Show this help message

Workflow sources:
  Embedded names:  kanban, todo, bug-tracker
  File path:       ./my-workflow.yaml, /path/to/workflow.yaml
  URL:             https://example.com/workflow.yaml

Examples:
  tiki init                              Initialize current directory interactively
  tiki init my-project                   Initialize my-project subdirectory
  tiki init -w todo                      Initialize with the todo workflow
  tiki init -w kanban test1              Initialize test1 with the kanban workflow
  tiki init -w ./custom.yaml             Initialize with a local workflow file
  tiki init -w https://example.com/w.yaml  Initialize with a remote workflow
  tiki init -n --samples                 Initialize non-interactively with sample tasks
  tiki init --ai-skill claude            Initialize with Claude Code AI skill
`)
}
