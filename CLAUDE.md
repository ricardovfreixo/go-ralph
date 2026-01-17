# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Build the binary
go build -o ralph ./cmd/ralph

# Run with a PRD file
./ralph PRD.md

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -v ./internal/parser -run TestParsePRD

# Format code
go fmt ./...

# Lint (if installed)
golangci-lint run
```

## Architecture

ralph-go is a TUI application that orchestrates multiple Claude Code instances to build applications autonomously from a PRD.

### Package Structure

- **cmd/ralph/** - Entry point, CLI argument handling
- **internal/tui/** - BubbleTea TUI implementation (Model, Update, View pattern)
- **internal/parser/** - PRD.md parsing (extracts features, tasks, metadata)
- **internal/runner/** - Claude Code instance management (start, stop, output streaming)
- **internal/state/** - Progress tracking and persistence (progress.md)

### Data Flow

1. User provides PRD.md path → `cmd/ralph/main.go`
2. TUI loads and parses PRD → `internal/parser/parser.go`
3. TUI displays features → `internal/tui/tui.go`
4. User starts feature → Runner spawns `claude` process → `internal/runner/runner.go`
5. Output streamed to TUI, state saved → `internal/state/state.go`

### TUI Pattern

Uses Charm's BubbleTea with the Elm Architecture:
- `Model` - Application state
- `Init()` - Initial commands (load PRD, load state)
- `Update(msg)` - Handle messages, return new model + commands
- `View()` - Render current state to string

Views: `viewMain` (feature list), `viewInspect` (instance output), `viewHelp`

### Claude Code Integration

Instances run with flags: `--dangerously-skip-permissions -p <prompt> --verbose --output-format stream-json`

Output is JSON-streamed and parsed for display. Message types: `assistant`, `tool_use`, `tool_result`, `system`, `error`.

## PRD Format

```markdown
# Title (H1 = context)
## Feature (H2 = feature)
Execution: sequential|parallel
Model: sonnet|opus|haiku
- [ ] Task item
Acceptance: criteria
```

## Conventions

- Use Go 1.25 features
- Keep TUI responsive - long operations in commands, not Update()
- State mutations through Progress methods (thread-safe)
- All runner output through channels, never blocking
