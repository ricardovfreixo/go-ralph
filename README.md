# ralph-go

Autonomous development orchestrator that runs Claude Code instances to build applications from a PRD.

Inspired by [Anthropic's Ralph Loop](https://github.com/anthropics/claude-plugins-official/blob/main/plugins/ralph-loop/README.md), ralph-go adds a TUI for monitoring and controlling the autonomous build process.

## How It Works

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   PRD.md    │────▶│  ralph-go   │────▶│  Your App   │
│  (features) │     │    (TUI)    │     │   (built)   │
└─────────────┘     └─────────────┘     └─────────────┘
                          │
                          ▼
                    Claude Code
                    instances
```

1. Write a PRD.md defining your project features
2. Run ralph-go to orchestrate Claude Code instances
3. Monitor progress, inspect outputs, interject if needed
4. Get a working application

## Installation

### Pre-built Binary (Linux amd64)

```bash
# Download latest
curl -L https://github.com/ricardovfreixo/go-ralph/raw/main/dist/v0.2.0/linux-amd64/ralph -o ralph
chmod +x ralph
sudo mv ralph /usr/local/bin/  # or ~/.local/bin/
```

### Build from Source

Requires Go 1.25+

```bash
git clone https://github.com/ricardovfreixo/go-ralph.git
cd ralph-go
go build -o ralph ./cmd/ralph
sudo mv ralph /usr/local/bin/  # or ~/.local/bin/
```

### Prerequisites

- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- Anthropic API access

## Quick Start

```bash
# Create project directory
mkdir my-app && cd my-app

# Initialize (creates .claude/CLAUDE.md, PRD.md template, input_design/)
ralph init

# Develop your PRD interactively with Claude
claude

# Run autonomous build
ralph PRD.md
```

## Commands

| Command | Description |
|---------|-------------|
| `ralph init` | Initialize new project with PRD template |
| `ralph <file>` | Run TUI with specified PRD file |
| `ralph help` | Show help |
| `ralph --version` | Show version |

## TUI Controls

**Main view:**

| Key | Action |
|-----|--------|
| `j/k` | Navigate features |
| `Enter` | Inspect feature output |
| `s` | Start feature |
| `S` | Start ALL (auto mode) |
| `r` | Retry failed feature |
| `R` | Reset feature |
| `x` | Stop feature |
| `X` | Stop ALL |
| `?` | Help |
| `q` | Quit (saves progress) |

**Inspect view:**

| Key | Action |
|-----|--------|
| `j/k` | Scroll |
| `g/G` | Top/bottom |
| `Esc` | Back |

## PRD Format

```markdown
# Project Title

Project context and tech stack.

## Feature 1: Name

Description of the feature.

Execution: sequential
Model: sonnet

- [ ] Task one
- [ ] Task two
- [ ] Write tests

Acceptance: What must be true when done
```

**Structure:**
- `#` (H1): Project context (shared with all features)
- `##` (H2): Individual features (each runs in separate Claude instance)
- `Execution`: `sequential` or `parallel`
- `Model`: `haiku`, `sonnet`, or `opus`
- Task lists: Checkboxes for items to implement
- `Acceptance:` Criteria for completion

## Project Files

| File | Purpose |
|------|---------|
| `PRD.md` | Project requirements (input) |
| `progress.md` | State tracking (auto-generated) |
| `input_design/` | Design assets |
| `.claude/CLAUDE.md` | PRD authoring guide for Claude |

## Documentation

| Document | Description |
|----------|-------------|
| [PRD Authoring Guide](docs/context.md) | How to write effective PRDs |
| [Roadmap](docs/ROADMAP.md) | Planned features and releases |
| [License](docs/LICENSE.md) | Source-available license terms |
| [Acknowledgments](docs/ACKNOWLEDGMENTS.md) | Credits and dependencies |
| [PRD Template](docs/examples/PRD.template.md) | Starting template for new projects |

## License

Source-available. Free to use and modify. No obligation to accept contributions or provide support. See [docs/LICENSE.md](docs/LICENSE.md).

## Status

Early stage. See [docs/ROADMAP.md](docs/ROADMAP.md) for planned features.
