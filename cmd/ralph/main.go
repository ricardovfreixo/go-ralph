package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/vx/ralph-go/internal/auto"
	ralphInit "github.com/vx/ralph-go/internal/init"
	"github.com/vx/ralph-go/internal/status"
	"github.com/vx/ralph-go/internal/tui"
	"github.com/vx/ralph-go/internal/tui/layout"
)

func main() {
	log.SetLevel(log.DebugLevel)

	if len(os.Args) < 2 {
		if auto.PRDDirExists() {
			runTUIManifest()
		} else {
			printUsage()
		}
		return
	}

	switch os.Args[1] {
	case "--help", "-h":
		printHelp()
		os.Exit(0)
	case "--version", "-v":
		fmt.Println(layout.AppName + " " + layout.AppVersion)
		os.Exit(0)
	case "--headless":
		if auto.PRDDirExists() {
			runAuto()
		} else {
			fmt.Println("Error: PRD/ directory not found. Run 'ralph init PRD.md' first.")
			os.Exit(1)
		}
	case "run":
		if auto.PRDDirExists() {
			runAuto()
		} else {
			fmt.Println("Error: PRD/ directory not found. Run 'ralph init PRD.md' first.")
			os.Exit(1)
		}
	case "init":
		runInit()
	case "status":
		runStatus()
	case "help":
		if len(os.Args) > 2 {
			printCommandHelp(os.Args[2])
		} else {
			printHelp()
		}
		os.Exit(0)
	default:
		runTUI(os.Args[1])
	}
}

func runAuto() {
	result, err := auto.Run()
	if err != nil {
		log.Error("Auto run failed", "error", err)
		fmt.Printf("\nError: %s\n", err)
		os.Exit(1)
	}

	auto.PrintSummary(result)
	os.Exit(auto.ExitCode(result))
}

func runStatus() {
	if err := status.Run(); err != nil {
		log.Fatal("Status failed", "error", err)
	}
}

func runInit() {
	force := false
	var prdPath string

	for _, arg := range os.Args[2:] {
		if arg == "--force" || arg == "-f" {
			force = true
		} else if !strings.HasPrefix(arg, "-") && prdPath == "" {
			prdPath = arg
		}
	}

	if prdPath != "" {
		if _, err := os.Stat(prdPath); os.IsNotExist(err) {
			log.Fatal("PRD file not found", "path", prdPath)
		}

		fmt.Printf("Initializing PRD directory structure from %s...\n", prdPath)
		fmt.Println()

		if err := ralphInit.InitFromPRD(prdPath, force); err != nil {
			log.Fatal("Init failed", "error", err)
		}

		fmt.Println()
		fmt.Println("PRD directory structure created successfully.")
		return
	}

	fmt.Println("Initializing ralph project...")
	fmt.Println()

	if err := ralphInit.Run(force); err != nil {
		log.Fatal("Init failed", "error", err)
	}
}

func runTUIManifest() {
	prdDir, err := auto.FindPRDDir()
	if err != nil {
		log.Fatal("Failed to find PRD directory", "error", err)
	}

	if err := tui.RunWithManifest(prdDir); err != nil {
		log.Fatal("Error running TUI", "error", err)
	}
}

func runTUI(prdPath string) {
	if _, err := os.Stat(prdPath); os.IsNotExist(err) {
		log.Fatal("PRD file not found", "path", prdPath)
	}

	// Check if PRD/ directory exists - if so, use manifest mode
	if auto.PRDDirExists() {
		prdDir, err := auto.FindPRDDir()
		if err == nil {
			if err := tui.RunWithManifest(prdDir); err != nil {
				log.Fatal("Error running TUI", "error", err)
			}
			return
		}
	}

	// Legacy mode - parse PRD file directly
	if err := tui.Run(prdPath); err != nil {
		log.Fatal("Error running TUI", "error", err)
	}
}

func printUsage() {
	fmt.Println(`ralph-go - Autonomous development orchestrator

Usage:
  ralph                   Run TUI (requires PRD/ directory)
  ralph run               Run next feature headless and exit
  ralph <PRD.md>          Run TUI (uses PRD/ if exists, else legacy mode)
  ralph status            Show current PRD progress
  ralph init <PRD.md>     Create PRD/ directory structure from PRD file
  ralph help [command]    Show help for a command

Workflow:
  ralph init PRD.md       # Create PRD/ directory structure
  ralph                   # Run TUI to manage features
  ralph run               # Or run single feature headless

Run 'ralph --help' for more information.`)
}

func printHelp() {
	fmt.Println(`ralph-go - Autonomous development orchestrator

Usage:
  ralph                         Run TUI (requires PRD/ directory)
  ralph run                     Run next feature headless and exit
  ralph --headless              Same as 'ralph run'
  ralph <PRD.md>                Run TUI (uses PRD/ if exists, else legacy mode)
  ralph status                  Show current PRD progress
  ralph init [--force]          Initialize a new ralph project in current directory
  ralph init <PRD.md> [--force] Create PRD/ directory structure from PRD file
  ralph help [command]          Show help for a command

Commands:
  (no args)   Run TUI if PRD/ exists, otherwise show usage
  run         Run next pending feature headless and exit
  status      Show feature status, dependencies, and progress summary
  init        Create project files, or generate PRD/ directory from PRD file
  help        Show help for a command

Options:
  -h, --help      Show this help message
  -v, --version   Show version
  --headless      Run headless mode (same as 'ralph run')

Workflow:

  ralph init PRD.md         # Create PRD/ directory structure
  ralph                     # Run TUI to manage features interactively
  ralph run                 # Or run single feature headless (for CI/scripts)
  ralph status              # Check progress

Headless Mode (ralph run):
  Finds the next runnable feature (respecting dependencies), runs it to
  completion, and exits. Useful for CI/CD or scripted execution.

  Exit codes:
    0 = Feature completed successfully, or no work to do
    1 = Feature failed

TUI Controls:
  j/k or ↑/↓    Navigate features
  Space         Expand/collapse child features
  Enter         Inspect running instance output
  s             Start selected feature
  S             Start ALL (auto mode)
  r             Retry failed/completed feature
  R             Reset feature (clear attempts)
  x             Stop running feature
  X             Stop ALL (exit auto mode)
  c             Toggle cost display
  ?             Show help
  q             Quit (saves progress)

Inspect View:
  j/k           Scroll (disables auto-scroll)
  g/G           Top/bottom
  f             Follow mode (auto-scroll)
  a             Toggle action timeline
  Esc           Back to main view

For more information, see the README.md file.`)
}

func printCommandHelp(cmd string) {
	switch cmd {
	case "status":
		fmt.Println(`ralph status - Show current PRD progress

Usage:
  ralph status

Displays a formatted overview of all features in the PRD/ directory including:
  - Feature status (pending, running, completed, failed, blocked)
  - Dependencies for each feature
  - Which dependencies are pending for blocked features
  - Summary counts of all feature states

Status icons:
  ✓  Completed
  ●  Running
  ✗  Failed
  ○  Pending (ready to run)
  ◌  Blocked (waiting on dependencies)`)
	case "init":
		fmt.Println(`ralph init - Initialize a ralph project or PRD directory structure

Usage:
  ralph init [--force]
  ralph init <PRD.md> [--force]

Without PRD file:
  Creates project scaffolding in the current directory:
    .claude/CLAUDE.md   PRD authoring guide for Claude Code
    PRD.md              Template PRD file to fill in
    input_design/       Directory for design assets

With PRD file:
  Creates PRD/ directory structure from existing PRD:
    PRD/01-feature-name/feature.md   First feature spec with global context
    PRD/02-feature-name/feature.md   Second feature spec with global context
    ...

Options:
  -f, --force   Overwrite existing files/directories

Workflow:
  1. Run 'ralph init' to create project template
  2. Edit PRD.md with your features
  3. Run 'ralph init PRD.md' to create directory structure
  4. Run 'ralph PRD.md' to start autonomous development`)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Run 'ralph --help' for available commands.")
	}
}
