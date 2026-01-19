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
			runAuto()
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

func runTUI(prdPath string) {
	if _, err := os.Stat(prdPath); os.IsNotExist(err) {
		log.Fatal("PRD file not found", "path", prdPath)
	}

	if err := tui.Run(prdPath); err != nil {
		log.Fatal("Error running TUI", "error", err)
	}
}

func printUsage() {
	fmt.Println(`ralph-go - Autonomous development orchestrator

Usage:
  ralph                   Run next pending feature (requires PRD/ directory)
  ralph <PRD.md>          Run interactive TUI with a PRD file (legacy mode)
  ralph status            Show current PRD progress
  ralph init              Initialize a new ralph project
  ralph init <PRD.md>     Create PRD/ directory structure from PRD file
  ralph help [command]    Show help for a command

Workflows:
  New workflow (autonomous):  ralph init PRD.md && ralph
  Legacy workflow (TUI):      ralph PRD.md

Run 'ralph --help' for more information.`)
}

func printHelp() {
	fmt.Println(`ralph-go - Autonomous development orchestrator

Usage:
  ralph                         Run next pending feature (requires PRD/ directory)
  ralph <PRD.md>                Run interactive TUI with a PRD file (legacy mode)
  ralph status                  Show current PRD progress
  ralph init [--force]          Initialize a new ralph project in current directory
  ralph init <PRD.md> [--force] Create PRD/ directory structure from PRD file
  ralph help [command]          Show help for a command

Commands:
  (no args)   Run next pending feature autonomously (if PRD/ exists), or show usage
  status      Show feature status, dependencies, and progress summary
  init        Create project files, or generate PRD/ directory from PRD file
  help        Show help for a command

Options:
  -h, --help      Show this help message
  -v, --version   Show version

Two Workflows:

  1. New Autonomous Workflow (recommended):
     Uses PRD/ directory structure with manifest-based dependency tracking.

     ralph init PRD.md         # Create PRD/ directory structure
     ralph                     # Run next pending feature and exit
     ralph status              # Check progress

  2. Legacy TUI Workflow:
     Interactive mode with real-time output, manual feature control.

     ralph PRD.md              # Launch TUI with PRD file

Autonomous Mode (no args):
  When run without arguments and PRD/ directory exists, ralph finds the next
  runnable feature (respecting dependencies), runs it to completion, and exits.
  If no PRD/ directory exists, shows usage help.

  Exit codes:
    0 = Feature completed successfully, or no work to do
    1 = Feature failed

TUI Controls (legacy mode):
  j/k or ↑/↓    Navigate features
  Enter         Inspect running instance output
  s             Start selected feature
  S             Start ALL (auto mode)
  r             Retry failed/completed feature
  R             Reset feature (clear attempts)
  x             Stop running feature
  X             Stop ALL (exit auto mode)
  ?             Show help
  q             Quit (saves progress)

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
