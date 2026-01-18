package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const claudeMDContent = `# Ralph PRD Authoring Guide

This project uses ralph-go for autonomous development. When helping create or edit PRD.md, follow these rules.

## PRD Structure

The PRD.md file must follow this exact structure:

### H1 (# Title) - Project Context
- Project name and description
- Technology stack decisions
- Architecture overview
- Coding standards

### H2 (## Feature Name) - Individual Features
Each feature is executed by a separate Claude Code instance.

**Required metadata (one per line, before tasks):**
- ` + "`Execution: sequential`" + ` or ` + "`Execution: parallel`" + ` - how tasks run
- ` + "`Model: sonnet`" + ` or ` + "`Model: opus`" + ` or ` + "`Model: haiku`" + ` - which model to use

**Task list format:**
` + "```markdown" + `
- [ ] Task description
- [ ] Another task
` + "```" + `

**Acceptance criteria format:**
` + "```markdown" + `
Acceptance: Description of what must be true when complete
Acceptance: Another criterion
` + "```" + `

## Example Feature

` + "```markdown" + `
## User Authentication

Implement JWT-based authentication for the API.

Execution: sequential
Model: sonnet

- [ ] Create User model with email and password_hash fields
- [ ] Implement POST /auth/register endpoint
- [ ] Implement POST /auth/login endpoint returning JWT
- [ ] Add auth middleware to validate tokens
- [ ] Write tests for all auth endpoints

Acceptance: Registration creates user and returns 201
Acceptance: Login returns valid JWT for correct credentials
Acceptance: Protected routes return 401 without valid token
Acceptance: All tests pass
` + "```" + `

## Best Practices

1. **Order features by dependency** - foundational features first
2. **Keep features independent** - minimize cross-feature dependencies
3. **Include tests in every feature** - each feature should verify itself
4. **Be specific** - vague tasks lead to inconsistent results
5. **Use acceptance criteria** - define what "done" means

## Model Selection Guide

- **haiku** - Simple, well-defined tasks (boilerplate, config files, simple CRUD)
- **sonnet** - Most coding tasks (default, good balance)
- **opus** - Complex architecture, nuanced decisions

## Design References

Place design files in ` + "`input_design/`" + `:
- Screenshots or mockups for UI
- Color schemes
- API schemas
- Architecture diagrams

Reference them in features:
` + "```markdown" + `
## Dashboard Page

Implement the dashboard following input_design/dashboard.png

Color scheme from input_design/colors.png
` + "```" + `

## After Creating PRD.md

Run ralph to start autonomous development:
` + "```bash" + `
ralph PRD.md
` + "```" + `

## Working Directory

All generated code and files are created in the same directory as the PRD file. This directory becomes the working directory for each Claude Code instance.

**Generated files:**
- ` + "`progress.json`" + ` - tracks feature status and attempts
- ` + "`.ralph/ralph.log`" + ` - runtime logs (useful for debugging)

Monitor activity: ` + "`tail -f .ralph/ralph.log`" + `
`

const prdTemplate = `# Project Name

Brief description of what this project does.

## Technology Stack

- Language:
- Framework:
- Database:

## Architecture Notes

High-level architecture decisions.

---

## Feature 1: Project Setup

Initialize the project structure.

Execution: sequential
Model: haiku

- [ ] Initialize project with package manager
- [ ] Set up directory structure
- [ ] Create configuration files

Acceptance: Project builds without errors

---

## Feature 2: Core Feature

Description of this feature.

Execution: sequential
Model: sonnet

- [ ] First task
- [ ] Second task
- [ ] Write tests

Acceptance: Tests pass
`

func appendToGitignore(path, entry string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := string(content)
	if strings.Contains(lines, entry) {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	_, err = f.WriteString(entry + "\n")
	return err
}

func Run(force bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	claudeDir := filepath.Join(cwd, ".claude")
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	prdFile := filepath.Join(cwd, "PRD.md")
	designDir := filepath.Join(cwd, "input_design")

	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	if _, err := os.Stat(claudeMD); err == nil && !force {
		fmt.Println("  .claude/CLAUDE.md already exists (use --force to overwrite)")
	} else {
		if err := os.WriteFile(claudeMD, []byte(claudeMDContent), 0644); err != nil {
			return fmt.Errorf("failed to write CLAUDE.md: %w", err)
		}
		fmt.Println("  Created .claude/CLAUDE.md")
	}

	if _, err := os.Stat(prdFile); err == nil && !force {
		fmt.Println("  PRD.md already exists (use --force to overwrite)")
	} else {
		if err := os.WriteFile(prdFile, []byte(prdTemplate), 0644); err != nil {
			return fmt.Errorf("failed to write PRD.md: %w", err)
		}
		fmt.Println("  Created PRD.md template")
	}

	if err := os.MkdirAll(designDir, 0755); err != nil {
		return fmt.Errorf("failed to create input_design directory: %w", err)
	}
	fmt.Println("  Created input_design/")

	gitignorePath := filepath.Join(cwd, ".gitignore")
	if err := appendToGitignore(gitignorePath, ".ralph/"); err != nil {
		fmt.Printf("  Warning: could not update .gitignore: %v\n", err)
	} else {
		fmt.Println("  Added .ralph/ to .gitignore")
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'claude' to interactively create your PRD.md")
	fmt.Println("  2. Add any design files to input_design/")
	fmt.Println("  3. Run 'ralph PRD.md' to start building")

	return nil
}
