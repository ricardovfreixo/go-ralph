# PRD Authoring Guide for ralph-go

This file provides guidance when creating a PRD.md file for use with ralph-go. Copy this file to your project directory and use it as context when working with Claude Code to create your PRD.

## Purpose

The PRD.md file is the master document that defines what ralph-go will build. It must be:
- **Complete**: All features and requirements specified
- **Structured**: Following the exact format ralph-go expects
- **Testable**: Each feature has clear acceptance criteria
- **Ordered**: Dependencies between features are respected

## PRD Structure

### H1: Project Context

The first heading provides overall context that every feature needs:

```markdown
# My Todo Application

A web-based todo application with user authentication, task management,
and real-time updates. Built with React frontend and Go backend.

Tech stack:
- Frontend: React 18, TypeScript, TailwindCSS
- Backend: Go 1.25, Chi router, PostgreSQL
- Auth: JWT tokens
- Deployment: Docker containers
```

**Include:**
- Project purpose and goals
- Technology stack decisions
- Architecture overview
- External service dependencies
- Coding standards or conventions

### H2: Features

Each feature is a unit of work for a Claude Code instance:

```markdown
## Feature 1: Database Schema and Migrations

Set up the PostgreSQL database with all required tables.

Execution: sequential
Model: sonnet

- [ ] Create users table with id, email, password_hash, created_at
- [ ] Create todos table with id, user_id, title, completed, created_at
- [ ] Create sessions table for JWT refresh tokens
- [ ] Write migration files using golang-migrate format
- [ ] Add seed data script for development

Acceptance: All migrations run without errors
Acceptance: Seed data creates a test user with sample todos
Acceptance: Foreign key constraints are properly defined
```

### Feature Metadata

**Execution Mode:**
- `sequential` - Tasks must be done in order (default)
- `parallel` - Tasks can be done simultaneously

**Model:**
- `sonnet` - Best for most coding tasks (default)
- `opus` - Complex architectural decisions
- `haiku` - Simple, repetitive tasks

### Task Lists

Tasks should be:
- **Atomic**: One clear deliverable per task
- **Verifiable**: Can be tested or checked
- **Ordered**: Listed in logical implementation order

```markdown
- [ ] Create the user model with validation
- [ ] Implement user registration endpoint
- [ ] Add password hashing with bcrypt
- [ ] Create login endpoint returning JWT
- [ ] Write integration tests for auth flow
```

### Acceptance Criteria

Define how to verify the feature is complete:

```markdown
Acceptance: All endpoints return proper HTTP status codes
Acceptance: Invalid input returns 400 with error details
Acceptance: Tests achieve 80% code coverage
Acceptance: No linter warnings
```

## Best Practices

### 1. Feature Independence

Each feature should be as independent as possible. If Feature B needs Feature A:
- List Feature A first
- Reference the dependency in Feature B's description

```markdown
## Feature 2: User Authentication

Requires: Feature 1 (Database Schema)

Implements user registration and login using the database schema from Feature 1.
```

### 2. Test-Driven Features

Always include testing tasks:

```markdown
- [ ] Write unit tests for business logic
- [ ] Write integration tests for API endpoints
- [ ] Ensure tests pass before completion
```

### 3. Progressive Complexity

Order features from foundational to advanced:

1. Project setup and configuration
2. Data models and database
3. Core business logic
4. API endpoints
5. Frontend components
6. Integration and polish

### 4. Design References

Place design files in `input_design/` and reference them:

```markdown
## Feature 5: Dashboard UI

Implement the main dashboard following the design in input_design/dashboard.png

Color scheme from input_design/colors.png:
- Primary: #3B82F6
- Secondary: #10B981
- Background: #F3F4F6
```

### 5. Clear Boundaries

Define what's in and out of scope:

```markdown
## Feature 3: Task Management API

CRUD operations for todo items.

In scope:
- Create, read, update, delete todos
- Filter by completion status
- Pagination

Out of scope (handled in Feature 6):
- Real-time updates
- Sharing todos with other users
```

## Validation Checklist

Before running ralph-go, verify your PRD:

- [ ] H1 provides complete project context
- [ ] Each H2 feature has a clear description
- [ ] All features have task lists with checkboxes
- [ ] Acceptance criteria are testable
- [ ] Features are ordered by dependency
- [ ] Execution mode specified where needed
- [ ] Model specified for complex/simple features
- [ ] Design files referenced are in input_design/

## Common Mistakes

**Too Vague:**
```markdown
- [ ] Implement the backend
```

**Better:**
```markdown
- [ ] Create Chi router with middleware stack
- [ ] Implement /api/v1/users endpoints
- [ ] Add request logging middleware
- [ ] Configure CORS for frontend origin
```

**Missing Context:**
```markdown
## User Profile Page
- [ ] Create profile component
```

**Better:**
```markdown
## User Profile Page

Display user information and allow profile updates.
Uses the existing UserContext from Feature 2.
Route: /profile (protected, requires authentication)

- [ ] Create ProfilePage component at src/pages/Profile.tsx
- [ ] Fetch user data using useUser hook
- [ ] Display avatar, name, email
- [ ] Add edit form for name and avatar
- [ ] Handle form submission with API call
```

## Example: Complete Feature

```markdown
## Feature 4: REST API Authentication Middleware

Implement JWT authentication middleware for protected routes.

Execution: sequential
Model: sonnet

Dependencies:
- Feature 1: Database schema (users, sessions tables)
- Feature 3: JWT token generation

Description:
Create middleware that validates JWT tokens on protected routes.
Extract user ID from valid tokens and attach to request context.
Handle token refresh using refresh tokens stored in sessions table.

- [ ] Create auth middleware in internal/middleware/auth.go
- [ ] Parse JWT from Authorization header (Bearer scheme)
- [ ] Validate token signature and expiration
- [ ] Extract user_id claim and fetch user from database
- [ ] Attach user to request context
- [ ] Create helper to get user from context
- [ ] Handle expired tokens with 401 response
- [ ] Implement token refresh endpoint POST /auth/refresh
- [ ] Write unit tests for middleware
- [ ] Write integration tests for protected routes

Acceptance: Protected routes return 401 without valid token
Acceptance: Valid tokens allow access to protected routes
Acceptance: Expired tokens trigger proper refresh flow
Acceptance: Tests cover all authentication scenarios
```

## Running ralph-go

Once your PRD is complete:

```bash
# Navigate to your project directory
cd my-project

# Ensure PRD.md and context.md are present
ls PRD.md input_design/

# Run ralph-go
ralph PRD.md
```

Monitor the TUI for progress and intervene if needed.
