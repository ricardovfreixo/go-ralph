package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	ralphInit "github.com/vx/ralph-go/internal/init"
	"github.com/vx/ralph-go/internal/tui"
)

func main() {
	log.SetLevel(log.DebugLevel)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "-h":
		printHelp()
		os.Exit(0)
	case "--version", "-v":
		fmt.Println("ralph-go v0.2.3")
		os.Exit(0)
	case "init":
		runInit()
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

func runInit() {
	force := false
	for _, arg := range os.Args[2:] {
		if arg == "--force" || arg == "-f" {
			force = true
		}
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
  ralph init              Initialize a new ralph project
  ralph <PRD.md>          Run ralph with the specified PRD file
  ralph help [command]    Show help for a command

Run 'ralph --help' for more information.`)
}

func printHelp() {
	fmt.Println(`ralph-go - Autonomous development orchestrator

Usage:
  ralph init [--force]    Initialize a new ralph project in current directory
  ralph <PRD.md>          Run ralph with the specified PRD file
  ralph help [command]    Show help for a command

Commands:
  init      Create .claude/CLAUDE.md, PRD.md template, and input_design/
  help      Show help for a command

Options:
  -h, --help      Show this help message
  -v, --version   Show version

Workflow:
  1. mkdir my-project && cd my-project
  2. ralph init
  3. claude                    # Interactively create PRD.md
  4. ralph PRD.md              # Run autonomous development

TUI Controls:
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
	case "init":
		fmt.Println(`ralph init - Initialize a new ralph project

Usage:
  ralph init [--force]

Creates the following in the current directory:
  .claude/CLAUDE.md   PRD authoring guide for Claude Code
  PRD.md              Template PRD file to fill in
  input_design/       Directory for design assets

Options:
  -f, --force   Overwrite existing files

After running init:
  1. Run 'claude' to interactively develop your PRD.md
  2. Add design mockups to input_design/
  3. Run 'ralph PRD.md' to start autonomous development`)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Run 'ralph --help' for available commands.")
	}
}
