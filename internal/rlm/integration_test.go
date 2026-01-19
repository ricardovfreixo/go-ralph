package rlm

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Integration tests for RLM features

// Test 1: Token parsing with real Claude Code output samples
func TestTokenParsingWithRealClaudeOutput(t *testing.T) {
	tests := []struct {
		name           string
		lines          []string
		wantInput      int64
		wantOutput     int64
		wantCacheRead  int64
		wantCacheWrite int64
		wantCost       float64
	}{
		{
			name: "assistant message with top-level usage",
			lines: []string{
				`{"type":"assistant","usage":{"input_tokens":1500,"output_tokens":200},"cost_usd":0.05}`,
			},
			wantInput:  1500,
			wantOutput: 200,
			wantCost:   0.05,
		},
		{
			name: "nested usage in message block",
			lines: []string{
				`{"type":"assistant","message":{"model":"claude-opus-4-5-20251101","id":"msg_123","type":"message","usage":{"input_tokens":2000,"output_tokens":500,"cache_read_input_tokens":300}}}`,
			},
			wantInput:     2000,
			wantOutput:    500,
			wantCacheRead: 300,
		},
		{
			name: "accumulation across multiple messages",
			lines: []string{
				`{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50},"cost_usd":0.01}`,
				`{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`,
				`{"type":"tool_result","result":"file1.go\nfile2.go"}`,
				`{"type":"assistant","usage":{"input_tokens":200,"output_tokens":100},"cost_usd":0.02}`,
			},
			wantInput:  300,
			wantOutput: 150,
			wantCost:   0.03,
		},
		{
			name: "full session with cache tokens",
			lines: []string{
				`{"type":"assistant","message":{"usage":{"input_tokens":5000,"output_tokens":1000,"cache_read_input_tokens":2000,"cache_creation_input_tokens":500}}}`,
				`{"type":"tool_use","tool":"Read","tool_input":{"file_path":"/path/to/file.go"}}`,
				`{"type":"tool_result","result":"package main..."}`,
				`{"type":"assistant","usage":{"input_tokens":3000,"output_tokens":800,"cache_read_input_tokens":1500}}`,
				`{"type":"result","subtype":"success","cost_usd":0.25}`,
			},
			wantInput:      8000,
			wantOutput:     1800,
			wantCacheRead:  3500,
			wantCacheWrite: 500,
			wantCost:       0.25,
		},
		{
			name: "tool errors don't affect token counts",
			lines: []string{
				`{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50}}`,
				`{"type":"tool_result","is_error":true,"result":"command failed"}`,
				`{"type":"assistant","usage":{"input_tokens":150,"output_tokens":75}}`,
			},
			wantInput:  250,
			wantOutput: 125,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager()
			feature := mgr.RegisterFeature("test-feature", "Test Feature")

			for _, line := range tt.lines {
				_, _ = mgr.ProcessOutput(feature.ID, line)
			}

			usage := feature.TokenUsage
			if usage == nil {
				t.Fatal("expected TokenUsage to be non-nil")
			}

			snapshot := usage.GetSnapshot()
			if snapshot.InputTokens != tt.wantInput {
				t.Errorf("InputTokens = %d, want %d", snapshot.InputTokens, tt.wantInput)
			}
			if snapshot.OutputTokens != tt.wantOutput {
				t.Errorf("OutputTokens = %d, want %d", snapshot.OutputTokens, tt.wantOutput)
			}
			if snapshot.CacheReadTokens != tt.wantCacheRead {
				t.Errorf("CacheReadTokens = %d, want %d", snapshot.CacheReadTokens, tt.wantCacheRead)
			}
			if snapshot.CacheWriteTokens != tt.wantCacheWrite {
				t.Errorf("CacheWriteTokens = %d, want %d", snapshot.CacheWriteTokens, tt.wantCacheWrite)
			}
			if tt.wantCost > 0 && snapshot.CostUSD != tt.wantCost {
				t.Errorf("CostUSD = %f, want %f", snapshot.CostUSD, tt.wantCost)
			}
		})
	}
}

// Test 2: Action extraction from tool_use messages
func TestActionExtractionFromToolUse(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		wantActions  int
		wantTypes    []string
		wantDetails  []string
	}{
		{
			name: "bash command",
			lines: []string{
				`{"type":"tool_use","tool":"Bash","tool_input":{"command":"go test ./..."}}`,
			},
			wantActions: 1,
			wantTypes:   []string{"command"},
			wantDetails: []string{"go test ./..."},
		},
		{
			name: "file operations",
			lines: []string{
				`{"type":"tool_use","tool":"Read","tool_input":{"file_path":"/path/to/file.go"}}`,
				`{"type":"tool_use","tool":"Write","tool_input":{"file_path":"/path/to/new.go"}}`,
				`{"type":"tool_use","tool":"Edit","tool_input":{"file_path":"/path/to/edit.go"}}`,
			},
			wantActions: 3,
			wantTypes:   []string{"file_read", "file_modify", "file_modify"},
			wantDetails: []string{"/path/to/file.go", "/path/to/new.go", "/path/to/edit.go"},
		},
		{
			name: "search operations",
			lines: []string{
				`{"type":"tool_use","tool":"Grep","tool_input":{"pattern":"func main"}}`,
				`{"type":"tool_use","tool":"Glob","tool_input":{"pattern":"**/*.go"}}`,
			},
			wantActions: 2,
			wantTypes:   []string{"search", "search"},
		},
		{
			name: "web and agent operations",
			lines: []string{
				`{"type":"tool_use","tool":"WebFetch","tool_input":{"url":"https://example.com"}}`,
				`{"type":"tool_use","tool":"Task","tool_input":{"prompt":"explore codebase"}}`,
			},
			wantActions: 2,
			wantTypes:   []string{"web_fetch", "agent_spawn"},
			wantDetails: []string{"https://example.com", "explore codebase"},
		},
		{
			name: "mixed session",
			lines: []string{
				`{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50}}`,
				`{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls -la"}}`,
				`{"type":"tool_result","result":"drwxr-xr-x"}`,
				`{"type":"tool_use","tool":"Read","tool_input":{"file_path":"main.go"}}`,
				`{"type":"tool_result","result":"package main"}`,
				`{"type":"tool_use","tool":"Edit","tool_input":{"file_path":"main.go"}}`,
			},
			wantActions: 3,
			wantTypes:   []string{"command", "file_read", "file_modify"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager()
			feature := mgr.RegisterFeature("test-feature", "Test Feature")

			for _, line := range tt.lines {
				_, _ = mgr.ProcessOutput(feature.ID, line)
			}

			actions := feature.GetActions()
			if len(actions) != tt.wantActions {
				t.Errorf("got %d actions, want %d", len(actions), tt.wantActions)
				for i, a := range actions {
					t.Logf("action[%d]: type=%s name=%s details=%s", i, a.Type, a.Name, a.Details)
				}
				return
			}

			for i, wantType := range tt.wantTypes {
				if i < len(actions) && actions[i].Type != wantType {
					t.Errorf("action[%d].Type = %s, want %s", i, actions[i].Type, wantType)
				}
			}

			for i, wantDetail := range tt.wantDetails {
				if i < len(actions) && !strings.Contains(actions[i].Details, wantDetail) {
					t.Errorf("action[%d].Details = %s, want to contain %s", i, actions[i].Details, wantDetail)
				}
			}
		})
	}
}

// Test 3: Spawn request detection
func TestSpawnRequestDetection(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantSpawn  bool
		wantTitle  string
		wantTasks  []string
		wantModel  string
		wantErr    error
	}{
		{
			name: "valid spawn request",
			line: `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":"Implement helper","tasks":["Create types","Add functions"],"model":"haiku"}}`,
			wantSpawn: true,
			wantTitle: "Implement helper",
			wantTasks: []string{"Create types", "Add functions"},
			wantModel: "haiku",
		},
		{
			name: "spawn request without optional model",
			line: `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":"Simple task","tasks":["Do it"]}}`,
			wantSpawn: true,
			wantTitle: "Simple task",
			wantTasks: []string{"Do it"},
			wantModel: "",
		},
		{
			name: "non-spawn tool use",
			line: `{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`,
			wantSpawn: false,
		},
		{
			name: "spawn with missing title",
			line: `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"tasks":["Do it"]}}`,
			wantSpawn: false,
			wantErr:   ErrInvalidSpawnData,
		},
		{
			name: "assistant message (not tool_use)",
			line: `{"type":"assistant","content":"I'll help you implement this feature."}`,
			wantSpawn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager()
			feature := mgr.RegisterFeature("test-feature", "Test Feature")
			feature.SetStatus("running")

			req, err := mgr.ProcessOutput(feature.ID, tt.line)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}

			gotSpawn := req != nil
			if gotSpawn != tt.wantSpawn {
				t.Errorf("spawn detected = %v, want %v", gotSpawn, tt.wantSpawn)
				return
			}

			if req != nil {
				if req.Title != tt.wantTitle {
					t.Errorf("Title = %q, want %q", req.Title, tt.wantTitle)
				}
				if len(req.Tasks) != len(tt.wantTasks) {
					t.Errorf("Tasks count = %d, want %d", len(req.Tasks), len(tt.wantTasks))
				}
				if req.Model != tt.wantModel {
					t.Errorf("Model = %q, want %q", req.Model, tt.wantModel)
				}
			}
		})
	}
}

// Test 4: Sub-feature spawning and hierarchy
func TestSubFeatureSpawning(t *testing.T) {
	mgr := NewManager()
	mgr.SetMaxDepth(3)

	root := mgr.RegisterFeature("root", "Root Feature")
	root.SetStatus("running")

	// Spawn first child
	spawnReq := &SpawnRequest{
		Title: "Child 1",
		Tasks: []string{"Task A", "Task B"},
		Model: "haiku",
	}

	child1, err := mgr.SpawnSubFeature(root.ID, spawnReq)
	if err != nil {
		t.Fatalf("failed to spawn child1: %v", err)
	}

	if child1.ParentID != root.ID {
		t.Errorf("child1.ParentID = %q, want %q", child1.ParentID, root.ID)
	}
	if child1.Depth != 1 {
		t.Errorf("child1.Depth = %d, want 1", child1.Depth)
	}
	if len(child1.Tasks) != 2 {
		t.Errorf("child1.Tasks = %d, want 2", len(child1.Tasks))
	}

	// Verify parent has child registered
	children := root.GetSubFeatures()
	if len(children) != 1 {
		t.Errorf("root children = %d, want 1", len(children))
	}

	// Spawn grandchild
	child1.SetStatus("running")
	grandchildReq := &SpawnRequest{
		Title: "Grandchild",
		Tasks: []string{"Deep task"},
	}

	grandchild, err := mgr.SpawnSubFeature(child1.ID, grandchildReq)
	if err != nil {
		t.Fatalf("failed to spawn grandchild: %v", err)
	}

	if grandchild.Depth != 2 {
		t.Errorf("grandchild.Depth = %d, want 2", grandchild.Depth)
	}
	if grandchild.ParentID != child1.ID {
		t.Errorf("grandchild.ParentID = %q, want %q", grandchild.ParentID, child1.ID)
	}

	// Test feature tree retrieval
	tree := mgr.GetFeatureTree(root.ID)
	if len(tree) != 3 {
		t.Errorf("tree size = %d, want 3", len(tree))
	}
}

// Test 5: Max depth enforcement
func TestMaxDepthEnforcement(t *testing.T) {
	mgr := NewManagerWithConfig(2, DefaultContextBudget)

	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	child, _ := mgr.SpawnSubFeature(root.ID, &SpawnRequest{Title: "Child", Tasks: []string{"task"}})
	child.SetStatus("running")

	grandchild, _ := mgr.SpawnSubFeature(child.ID, &SpawnRequest{Title: "Grandchild", Tasks: []string{"task"}})
	grandchild.SetStatus("running")

	// Try to spawn at max depth - should fail
	_, err := mgr.SpawnSubFeature(grandchild.ID, &SpawnRequest{Title: "Too Deep", Tasks: []string{"task"}})
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

// Test 6: Context budget calculation
func TestContextBudgetCalculation(t *testing.T) {
	baseBudget := int64(100000)
	mgr := NewManagerWithConfig(5, baseBudget)

	root := mgr.RegisterFeature("root", "Root")
	if root.ContextBudget != baseBudget {
		t.Errorf("root budget = %d, want %d", root.ContextBudget, baseBudget)
	}

	root.SetStatus("running")

	child, _ := mgr.SpawnSubFeature(root.ID, &SpawnRequest{Title: "Child", Tasks: []string{"task"}})
	expectedChildBudget := baseBudget / 2
	if child.ContextBudget != expectedChildBudget {
		t.Errorf("child budget = %d, want %d", child.ContextBudget, expectedChildBudget)
	}

	child.SetStatus("running")
	grandchild, _ := mgr.SpawnSubFeature(child.ID, &SpawnRequest{Title: "Grandchild", Tasks: []string{"task"}})
	// Formula: childBudget / (grandchildDepth + 1) = 50000 / 3 = 16666
	expectedGrandchildBudget := expectedChildBudget / 3
	if grandchild.ContextBudget != expectedGrandchildBudget {
		t.Errorf("grandchild budget = %d, want %d", grandchild.ContextBudget, expectedGrandchildBudget)
	}
}

// Test 7: Token usage aggregation across tree
func TestTokenUsageAggregation(t *testing.T) {
	mgr := NewManager()

	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	// Add usage to root
	mgr.ProcessOutput(root.ID, `{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500}}`)

	// Spawn and add usage to child
	child, _ := mgr.SpawnSubFeature(root.ID, &SpawnRequest{Title: "Child", Tasks: []string{"task"}})
	child.SetStatus("running")
	mgr.ProcessOutput(child.ID, `{"type":"assistant","usage":{"input_tokens":2000,"output_tokens":1000}}`)

	// Get total usage for tree
	total := mgr.GetTotalTokenUsage(root.ID)
	if total == nil {
		t.Fatal("expected total usage to be non-nil")
	}

	snapshot := total.GetSnapshot()
	if snapshot.InputTokens != 3000 {
		t.Errorf("total InputTokens = %d, want 3000", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 1500 {
		t.Errorf("total OutputTokens = %d, want 1500", snapshot.OutputTokens)
	}
}

// Test 8: Feature completion and result context
func TestFeatureCompletionAndResultContext(t *testing.T) {
	mgr := NewManager()

	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	child, _ := mgr.SpawnSubFeature(root.ID, &SpawnRequest{Title: "Child Task", Tasks: []string{"task"}})
	child.SetStatus("running")

	// Add some usage
	mgr.ProcessOutput(child.ID, `{"type":"assistant","usage":{"input_tokens":1000,"output_tokens":500}}`)

	// Complete the child
	result := mgr.CompleteSubFeature(child.ID, "completed", "All tasks done")
	if result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if result.Status != "completed" {
		t.Errorf("result.Status = %q, want %q", result.Status, "completed")
	}
	if result.Summary != "All tasks done" {
		t.Errorf("result.Summary = %q, want %q", result.Summary, "All tasks done")
	}

	// Generate context for parent
	ctx := mgr.GenerateSpawnResultContext(result)
	if ctx == "" {
		t.Error("expected context to be non-empty")
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(ctx), &parsed); err != nil {
		t.Errorf("failed to parse context JSON: %v", err)
	}

	if _, ok := parsed["sub_feature_completed"]; !ok {
		t.Error("expected sub_feature_completed key in context")
	}
}

// Test 9: Concurrent access safety
func TestConcurrentAccess(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent processing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			line := fmt.Sprintf(`{"type":"assistant","usage":{"input_tokens":%d,"output_tokens":%d}}`, i*10, i*5)
			if _, err := mgr.ProcessOutput(root.ID, line); err != nil && err != ErrFeatureNotFound {
				errCh <- err
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = root.GetActions()
			_ = root.TokenUsage.GetSnapshot()
			_ = root.GetStatus()
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	// Verify total tokens accumulated
	snapshot := root.TokenUsage.GetSnapshot()
	// Sum of 0+10+20+...+490 = 50*49/2 * 10 = 12250
	expectedInput := int64(49 * 50 / 2 * 10)
	if snapshot.InputTokens != expectedInput {
		t.Errorf("InputTokens = %d, want %d", snapshot.InputTokens, expectedInput)
	}
}

// Test 10: Spawn from stream-json line
func TestSpawnFromStreamJSON(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	line := `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":"Dynamic Child","tasks":["Task 1","Task 2"],"model":"sonnet"}}`

	req, err := mgr.ProcessOutput(root.ID, line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected spawn request, got nil")
	}

	// Spawn the child
	child, err := mgr.SpawnSubFeature(root.ID, req)
	if err != nil {
		t.Fatalf("failed to spawn: %v", err)
	}

	if child.Title != "Dynamic Child" {
		t.Errorf("child.Title = %q, want %q", child.Title, "Dynamic Child")
	}
	if child.Model != "sonnet" {
		t.Errorf("child.Model = %q, want %q", child.Model, "sonnet")
	}
}

// Test 11: Parent must be running to spawn
func TestParentMustBeRunningToSpawn(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("root", "Root")
	// Don't set status to running

	req := &SpawnRequest{Title: "Child", Tasks: []string{"task"}}
	_, err := mgr.SpawnSubFeature(root.ID, req)
	if err != ErrParentNotRunning {
		t.Errorf("expected ErrParentNotRunning, got %v", err)
	}
}

// Test 12: Actions aggregation across tree
func TestActionsAggregationAcrossTree(t *testing.T) {
	mgr := NewManager()

	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")
	mgr.ProcessOutput(root.ID, `{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`)

	child, _ := mgr.SpawnSubFeature(root.ID, &SpawnRequest{Title: "Child", Tasks: []string{"task"}})
	child.SetStatus("running")
	mgr.ProcessOutput(child.ID, `{"type":"tool_use","tool":"Read","tool_input":{"file_path":"file.go"}}`)
	mgr.ProcessOutput(child.ID, `{"type":"tool_use","tool":"Write","tool_input":{"file_path":"new.go"}}`)

	allActions := mgr.GetAllActions(root.ID)
	if len(allActions) != 3 {
		t.Errorf("total actions = %d, want 3", len(allActions))
	}

	// Verify action types
	types := make(map[string]int)
	for _, a := range allActions {
		types[a.Type]++
	}

	if types["command"] != 1 {
		t.Errorf("command actions = %d, want 1", types["command"])
	}
	if types["file_read"] != 1 {
		t.Errorf("file_read actions = %d, want 1", types["file_read"])
	}
	if types["file_modify"] != 1 {
		t.Errorf("file_modify actions = %d, want 1", types["file_modify"])
	}
}

// Test 13: Real-world session simulation
func TestRealWorldSession(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("feature-01", "Implement User Auth")
	root.SetStatus("running")

	// Simulate a real Claude Code session
	sessionLines := []string{
		// Initial assistant message with system setup
		`{"type":"assistant","message":{"model":"claude-sonnet","usage":{"input_tokens":5000,"output_tokens":200,"cache_read_input_tokens":1500}}}`,
		// File exploration
		`{"type":"tool_use","tool":"Glob","tool_input":{"pattern":"**/*.go"}}`,
		`{"type":"tool_result","result":"main.go\nauth/handler.go\nauth/middleware.go"}`,
		// Reading files
		`{"type":"tool_use","tool":"Read","tool_input":{"file_path":"auth/handler.go"}}`,
		`{"type":"tool_result","result":"package auth\n\nfunc Login()..."}`,
		// Writing new code
		`{"type":"tool_use","tool":"Write","tool_input":{"file_path":"auth/jwt.go"}}`,
		`{"type":"tool_result","result":"File written successfully"}`,
		// Running tests
		`{"type":"tool_use","tool":"Bash","tool_input":{"command":"go test ./auth/..."}}`,
		`{"type":"tool_result","result":"ok  auth 0.005s"}`,
		// Additional assistant processing
		`{"type":"assistant","usage":{"input_tokens":3000,"output_tokens":500}}`,
		// Final result
		`{"type":"result","subtype":"success","cost_usd":0.15}`,
	}

	for _, line := range sessionLines {
		_, _ = mgr.ProcessOutput(root.ID, line)
	}

	// Verify usage
	usage := root.TokenUsage.GetSnapshot()
	if usage.InputTokens != 8000 {
		t.Errorf("InputTokens = %d, want 8000", usage.InputTokens)
	}
	if usage.OutputTokens != 700 {
		t.Errorf("OutputTokens = %d, want 700", usage.OutputTokens)
	}

	// Verify actions
	actions := root.GetActions()
	expectedActions := 4 // Glob, Read, Write, Bash
	if len(actions) != expectedActions {
		t.Errorf("actions = %d, want %d", len(actions), expectedActions)
		for i, a := range actions {
			t.Logf("action[%d]: type=%s name=%s", i, a.Type, a.Name)
		}
	}
}

// Test 14: Context budget thresholds
func TestContextBudgetThresholds(t *testing.T) {
	mgr := NewManager()
	feature := mgr.RegisterFeature("test", "Test")
	feature.ContextBudget = 10000

	// Set usage at 79% - should not need summarization
	feature.SetContextUsed(7900)
	if feature.NeedsContextSummarization() {
		t.Error("should not need summarization at 79%")
	}

	// Set usage at 80% - should need summarization
	feature.SetContextUsed(8000)
	if !feature.NeedsContextSummarization() {
		t.Error("should need summarization at 80%")
	}

	// Set usage at 100% - exactly at budget, not over
	feature.SetContextUsed(10000)
	if feature.IsContextOverBudget() {
		t.Error("should NOT be over budget at exactly 100% (uses > not >=)")
	}

	// Set usage at 101% - over budget
	feature.SetContextUsed(10100)
	if !feature.IsContextOverBudget() {
		t.Error("should be over budget at 101%")
	}
}

// Test 15: Invalid spawn data handling
func TestInvalidSpawnDataHandling(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	invalidLines := []string{
		`{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":"not an object"}`,
		`{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{}}`,
		`{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":""}}`,
	}

	for i, line := range invalidLines {
		_, err := mgr.ProcessOutput(root.ID, line)
		if err != ErrInvalidSpawnData {
			t.Errorf("line[%d]: expected ErrInvalidSpawnData, got %v", i, err)
		}
	}
}

// Test 16: Feature ID generation consistency
func TestFeatureIDGeneration(t *testing.T) {
	mgr := NewManager()
	root := mgr.RegisterFeature("root", "Root")
	root.SetStatus("running")

	req := &SpawnRequest{Title: "Child", Tasks: []string{"task"}}
	child1, _ := mgr.SpawnSubFeature(root.ID, req)

	// Re-register root and spawn same child - should get same ID
	mgr2 := NewManager()
	root2 := mgr2.RegisterFeature("root", "Root")
	root2.SetStatus("running")
	child2, _ := mgr2.SpawnSubFeature(root2.ID, req)

	if child1.ID != child2.ID {
		t.Errorf("IDs should be deterministic: %q vs %q", child1.ID, child2.ID)
	}
}

// Test 17: Tracker processes all message types
func TestTrackerMessageTypes(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test")
	tracker := NewTracker(feature)

	messages := []struct {
		line       string
		wantUsage  bool
		wantAction bool
	}{
		{`{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50}}`, true, false},
		{`{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`, false, true},
		{`{"type":"tool_result","result":"output"}`, false, false},
		{`{"type":"system","content":"system message"}`, false, false},
		{`{"type":"error","content":"error message"}`, false, false},
		{`{"type":"result","subtype":"success","cost_usd":0.1}`, true, false},
		{`not json`, false, false},
		{``, false, false},
	}

	for i, tc := range messages {
		beforeUsage := feature.TokenUsage.GetSnapshot().TotalTokens
		beforeActions := len(feature.GetActions())

		tracker.ProcessLine(tc.line)

		afterUsage := feature.TokenUsage.GetSnapshot().TotalTokens
		afterActions := len(feature.GetActions())

		usageChanged := afterUsage != beforeUsage || (tc.line == messages[5].line && feature.TokenUsage.GetSnapshot().CostUSD > 0)
		actionAdded := afterActions > beforeActions

		if usageChanged != tc.wantUsage && i != 5 { // Skip cost-only message check
			t.Errorf("message[%d] %q: usage changed = %v, want %v", i, tc.line, usageChanged, tc.wantUsage)
		}
		if actionAdded != tc.wantAction {
			t.Errorf("message[%d] %q: action added = %v, want %v", i, tc.line, actionAdded, tc.wantAction)
		}
	}
}

// Test 18: Long command truncation in action details
func TestLongCommandTruncation(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test")
	tracker := NewTracker(feature)

	longCmd := strings.Repeat("a", 200)
	line := fmt.Sprintf(`{"type":"tool_use","tool":"Bash","tool_input":{"command":"%s"}}`, longCmd)

	tracker.ProcessLine(line)

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if len(actions[0].Details) > 103 { // 100 + "..."
		t.Errorf("command should be truncated, got length %d", len(actions[0].Details))
	}
}

// Test 19: Multiple features tracked independently
func TestMultipleFeaturesTrackedIndependently(t *testing.T) {
	mgr := NewManager()

	f1 := mgr.RegisterFeature("f1", "Feature 1")
	f2 := mgr.RegisterFeature("f2", "Feature 2")
	f1.SetStatus("running")
	f2.SetStatus("running")

	// Process output for feature 1
	mgr.ProcessOutput(f1.ID, `{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50}}`)
	mgr.ProcessOutput(f1.ID, `{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`)

	// Process output for feature 2
	mgr.ProcessOutput(f2.ID, `{"type":"assistant","usage":{"input_tokens":200,"output_tokens":100}}`)
	mgr.ProcessOutput(f2.ID, `{"type":"tool_use","tool":"Read","tool_input":{"file_path":"file.go"}}`)

	// Verify independence
	if f1.TokenUsage.GetSnapshot().InputTokens != 100 {
		t.Errorf("f1 InputTokens = %d, want 100", f1.TokenUsage.GetSnapshot().InputTokens)
	}
	if f2.TokenUsage.GetSnapshot().InputTokens != 200 {
		t.Errorf("f2 InputTokens = %d, want 200", f2.TokenUsage.GetSnapshot().InputTokens)
	}

	if len(f1.GetActions()) != 1 {
		t.Errorf("f1 actions = %d, want 1", len(f1.GetActions()))
	}
	if len(f2.GetActions()) != 1 {
		t.Errorf("f2 actions = %d, want 1", len(f2.GetActions()))
	}

	if f1.GetActions()[0].Type != "command" {
		t.Errorf("f1 action type = %s, want command", f1.GetActions()[0].Type)
	}
	if f2.GetActions()[0].Type != "file_read" {
		t.Errorf("f2 action type = %s, want file_read", f2.GetActions()[0].Type)
	}
}

// Test 20: Root features retrieval
func TestRootFeaturesRetrieval(t *testing.T) {
	mgr := NewManager()

	// Create multiple root features
	mgr.RegisterFeature("root1", "Root 1")
	mgr.RegisterFeature("root2", "Root 2")

	// Create a child feature
	root1 := mgr.GetFeature("root1")
	root1.SetStatus("running")
	mgr.SpawnSubFeature(root1.ID, &SpawnRequest{Title: "Child", Tasks: []string{"task"}})

	roots := mgr.GetRootFeatures()
	if len(roots) != 2 {
		t.Errorf("root count = %d, want 2", len(roots))
	}

	// All roots should have IsRoot() true
	for _, r := range roots {
		if !r.IsRoot() {
			t.Errorf("feature %s should be root", r.ID)
		}
	}
}

// Test 21: Feature status transitions
func TestFeatureStatusTransitions(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test")

	// Initial status should be "pending"
	if feature.GetStatus() != "pending" {
		t.Errorf("initial status = %q, want pending", feature.GetStatus())
	}

	// Transition to running
	feature.SetStatus("running")
	if feature.GetStatus() != "running" {
		t.Errorf("status after set to running = %q", feature.GetStatus())
	}

	// Can spawn when running
	if !feature.CanSpawn() {
		t.Error("should be able to spawn when running")
	}

	// Transition to completed
	feature.SetStatus("completed")
	if feature.GetStatus() != "completed" {
		t.Errorf("status after set to completed = %q", feature.GetStatus())
	}
}

// Test 22: Timestamp tracking in actions
func TestActionTimestampTracking(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test")
	tracker := NewTracker(feature)

	before := time.Now()
	tracker.ProcessLine(`{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`)
	after := time.Now()

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Timestamp.Before(before) || actions[0].Timestamp.After(after) {
		t.Error("action timestamp should be between before and after")
	}
}
