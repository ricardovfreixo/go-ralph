# Changelog

All notable changes to Ralph will be documented in this file.

## [v0.5.1] - 2026-01-23

### Fixed
- **TUI auto mode now respects feature dependencies** - Previously, pressing "S" (Start ALL) would start features without checking the `Depends:` field, potentially running features before their dependencies completed. Now uses `manifest.GetNextRunnableFeature()` which only returns features whose dependencies have all completed.
- **Manifest status synchronization** - Feature status changes (running/completed/failed) are now properly written to `manifest.json`, ensuring the dependency graph stays accurate during execution.

## [v0.5.0] - 2026-01-20

### Added
- Token and cost tracking persisted to FeatureState (survives instance cleanup)
- Aggregated totals from completed features (state) + running instances (manager)
- Elapsed time display per feature and total in header

## [v0.4.2] - 2026-01-19

### Fixed
- Nil pointer panic on startup - added nil checks for `m.state` and `m.prd` in View() render path before async load completes

## [v0.4.1] - 2026-01-19

### Added
- TUI manifest mode - uses PRD/ directory structure created by `ralph init`
- Feature status tracking via manifest.json
- Dependency declarations with `Depends:` field in PRD features

## [v0.4.0] - 2026-01-18

### Added
- RLM (Recursive Language Model) integration
- Recursive child features with token tracking
- Auto model selection based on task complexity
