# Roadmap

## v0.1.0 - Foundation ✓

- [x] Project structure and Go module setup
- [x] Basic TUI with BubbleTea
- [x] PRD parser for markdown format
- [x] State management with progress.md
- [x] Runner for Claude Code instances
- [x] Stream JSON output parsing
- [x] Feature list navigation
- [x] Instance output inspection view
- [x] Start/stop feature execution
- [x] Retry failed/completed features
- [x] Output scrolling (j/k/g/G in inspect view)

## v0.2.0 - Core Functionality ✓

- [x] Full Claude Code integration (stream-json parsing)
- [x] Parallel feature execution (up to 3 concurrent)
- [x] Sequential task ordering within features
- [x] Test result parsing from output
- [x] Automatic retry on failure (up to 3 attempts)
- [x] Progress persistence across restarts (JSON format)
- [x] Auto mode (S key) for hands-off execution

## v0.3.0 - Workflow Enhancement ✓

- [x] PRD directory structure (`PRD/` with numbered feature dirs)
- [x] Manifest generation and parsing (`manifest.json`)
- [x] Feature dependency resolution with cycle detection
- [x] DAG-based execution ordering
- [x] `ralph init <prd.md>` command
- [x] `ralph status` command
- [x] Autonomous single-feature execution (`ralph` with no args)
- [x] Auto-archive PRD on completion
- [x] Backward compatibility with legacy single-file mode

## v0.4.0 - Intelligence (RLM Integration) ✓

Inspired by [Recursive Language Models](https://arxiv.org/abs/2512.24601) research.

### TUI Enhancements
- [x] Inspect view auto-scroll (follows new output like `tail -f`)
- [x] Significant action extraction (files, commands, agents, fetches)
- [x] Action timeline view (toggle with 'a' key)
- [x] Compact action summary in feature list

### Token & Cost Tracking
- [x] Token usage parsing from stream-json
- [x] Usage display in TUI (compact and detailed)
- [x] Cost estimation per model
- [x] Budget limits in PRD (`Budget: $5.00` or `Tokens: 100000`)
- [x] Budget pause at 90% threshold with user confirmation

### Recursive Task Decomposition
- [x] Features can spawn sub-features dynamically
- [x] Hierarchical progress tree (parent → child tasks)
- [x] Bounded context per recursion level
- [x] Result summarization between layers
- [x] Sub-feature data model with parent/child relationships

### Dynamic Model Selection
- [x] `Model: auto` - start cheap, escalate on complexity
- [x] Haiku for simple/leaf tasks, Sonnet/Opus for complex decisions
- [x] Model escalation triggers (errors, complexity detection)
- [x] Model switch tracking and display

### Fault Tolerance
- [x] Child failures isolated from parent (configurable isolation level)
- [x] Automatic retry with adjusted parameters
- [x] Model escalation before retry
- [x] Adjustment history tracking

## v0.5.0 - Extended Orchestration ✓

### Aggregated Metrics
- [x] Total token usage for entire PRD (header display)
- [x] Total cost aggregation across all features
- [x] Elapsed time per feature (start → complete)
- [x] Total elapsed time for PRD execution

### Deferred to Future
- [ ] Context sharing between sibling instances
- [ ] Memory of previous runs
- [ ] Parallel subtask execution with result merging

## v0.6.0 - Interactive Features

### Task Control
- [ ] Mark tasks as "done" (skip execution, count as complete)
- [ ] Mark tasks as "skip" (exclude from execution entirely)
- [ ] Persist done/skip state across restarts

### Live Editing
- [ ] Pause/resume running features
- [ ] Add tasks during execution
- [ ] Edit feature descriptions live
- [ ] Manual interjection points
- [ ] Output search

## v0.7.0 - Polish

- [ ] Configurable themes
- [ ] Keyboard shortcut customization
- [ ] Export reports (HTML/PDF)
- [ ] Webhook notifications (Slack, Discord)
- [ ] Detailed cost breakdown report

## v0.8.0 - Ralph Server

Web-based project management interface.

### Core
- [ ] HTTP server with REST API
- [ ] Project registration (directory → project mapping)
- [ ] PRD file watching and auto-reload
- [ ] WebSocket for live status updates

### Dashboard
- [ ] Multi-project overview
- [ ] PRD viewer/editor
- [ ] Feature execution controls (start/stop/retry)
- [ ] Live output streaming
- [ ] Token usage and cost analytics

### Project Management
- [ ] Add/remove features from PRD via UI
- [ ] Reorder feature priority
- [ ] Archive completed projects
- [ ] Execution history and logs

## Future Ideas

- Team collaboration features
- CI/CD integration
- Plugin system for custom validators
- Paid API fallback when subscription exhausted
- Mobile-friendly responsive UI
