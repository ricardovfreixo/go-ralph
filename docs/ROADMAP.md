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

## v0.3.0 - Intelligence (RLM Integration)

Inspired by [Recursive Language Models](https://arxiv.org/abs/2512.24601) research.

### Recursive Task Decomposition
- [ ] Features can spawn sub-features dynamically
- [ ] Hierarchical progress tree (parent → child tasks)
- [ ] Bounded context per recursion level
- [ ] Result summarization between layers

### Smart Resource Management
- [ ] Token usage tracking per instance
- [ ] Cumulative token/cost accounting across recursion tree
- [ ] Usage budget in PRD (`Budget: $5.00` or `Tokens: 100000`)
- [ ] Pause when approaching Claude Code subscription limits
- [ ] Wait for limit replenishment or switch to API (configurable)

### Dynamic Model Selection
- [ ] `Model: auto` - start cheap, escalate on complexity
- [ ] Haiku for simple subtasks, Opus for complex decisions
- [ ] Model escalation triggers (errors, complexity detection)

### Fault Tolerance
- [ ] Subtask failures isolated from parent
- [ ] Automatic retry with adjusted parameters
- [ ] Fallback strategies (simplify task, use different model)

## v0.4.0 - Orchestration

- [ ] Dependency detection between features
- [ ] Smart execution ordering (DAG-based)
- [ ] Context sharing between sibling instances
- [ ] Memory of previous runs
- [ ] Parallel subtask execution with result merging

## v0.5.0 - Interactive Features

- [ ] Pause/resume running features
- [ ] Add tasks during execution
- [ ] Edit feature descriptions live
- [ ] Manual interjection points
- [ ] Output search

## v0.6.0 - Polish

- [ ] Configurable themes
- [ ] Keyboard shortcut customization
- [ ] Export reports (HTML/PDF)
- [ ] Webhook notifications (Slack, Discord)
- [ ] Detailed cost breakdown report

## Future Ideas

- Web UI alternative
- Multi-project dashboard
- Team collaboration features
- CI/CD integration
- Plugin system for custom validators
- Paid API fallback when subscription exhausted
