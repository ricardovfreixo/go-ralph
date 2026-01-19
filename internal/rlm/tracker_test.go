package rlm

import (
	"encoding/json"
	"testing"
)

func TestTrackerProcessLineUsage(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50},"cost_usd":0.01}`

	_, err := tracker.ProcessLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := feature.TokenUsage.GetSnapshot()
	if snapshot.InputTokens != 100 {
		t.Errorf("expected InputTokens 100, got %d", snapshot.InputTokens)
	}
	if snapshot.OutputTokens != 50 {
		t.Errorf("expected OutputTokens 50, got %d", snapshot.OutputTokens)
	}
}

func TestTrackerProcessLineNestedUsage(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	msg := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":  200,
			"output_tokens": 100,
		},
	}
	msgBytes, _ := json.Marshal(msg)

	line := `{"type":"assistant","message":` + string(msgBytes) + `}`

	_, err := tracker.ProcessLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := feature.TokenUsage.GetSnapshot()
	if snapshot.InputTokens != 200 {
		t.Errorf("expected InputTokens 200, got %d", snapshot.InputTokens)
	}
}

func TestTrackerProcessLineCacheTokens(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":30,"cache_creation_input_tokens":20}}`

	_, err := tracker.ProcessLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := feature.TokenUsage.GetSnapshot()
	if snapshot.CacheReadTokens != 30 {
		t.Errorf("expected CacheReadTokens 30, got %d", snapshot.CacheReadTokens)
	}
	if snapshot.CacheWriteTokens != 20 {
		t.Errorf("expected CacheWriteTokens 20, got %d", snapshot.CacheWriteTokens)
	}
}

func TestTrackerExtractActionFileModify(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"Write","tool_input":{"file_path":"/path/to/file.go"}}`

	_, err := tracker.ProcessLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Type != "file_modify" {
		t.Errorf("expected action type 'file_modify', got '%s'", actions[0].Type)
	}
	if actions[0].Name != "Write" {
		t.Errorf("expected action name 'Write', got '%s'", actions[0].Name)
	}
	if actions[0].Details != "/path/to/file.go" {
		t.Errorf("expected details '/path/to/file.go', got '%s'", actions[0].Details)
	}
}

func TestTrackerExtractActionBash(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"Bash","tool_input":{"command":"go test ./..."}}`

	tracker.ProcessLine(line)

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Type != "command" {
		t.Errorf("expected action type 'command', got '%s'", actions[0].Type)
	}
	if actions[0].Details != "go test ./..." {
		t.Errorf("expected details 'go test ./...', got '%s'", actions[0].Details)
	}
}

func TestTrackerExtractActionWebFetch(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"WebFetch","tool_input":{"url":"https://example.com"}}`

	tracker.ProcessLine(line)

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Type != "web_fetch" {
		t.Errorf("expected action type 'web_fetch', got '%s'", actions[0].Type)
	}
}

func TestTrackerExtractActionAgentSpawn(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"Task","tool_input":{"prompt":"explore the codebase"}}`

	tracker.ProcessLine(line)

	actions := feature.GetActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Type != "agent_spawn" {
		t.Errorf("expected action type 'agent_spawn', got '%s'", actions[0].Type)
	}
}

func TestTrackerExtractActionSearch(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	lines := []string{
		`{"type":"tool_use","tool":"Grep","tool_input":{"pattern":"func main"}}`,
		`{"type":"tool_use","tool":"Glob","tool_input":{"pattern":"*.go"}}`,
	}

	for _, line := range lines {
		tracker.ProcessLine(line)
	}

	actions := feature.GetActions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	for _, a := range actions {
		if a.Type != "search" {
			t.Errorf("expected action type 'search', got '%s'", a.Type)
		}
	}
}

func TestTrackerDetectSpawnRequest(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)
	feature.SetStatus("running")

	line := `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":"Helper Module","tasks":["Create types","Add functions"],"model":"haiku"}}`

	req, err := tracker.ProcessLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req == nil {
		t.Fatal("expected spawn request, got nil")
	}

	if req.Title != "Helper Module" {
		t.Errorf("expected title 'Helper Module', got '%s'", req.Title)
	}
	if len(req.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(req.Tasks))
	}
	if req.Model != "haiku" {
		t.Errorf("expected model 'haiku', got '%s'", req.Model)
	}
}

func TestTrackerDetectSpawnRequestMaxDepthExceeded(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	feature.Depth = 5
	feature.MaxDepth = 5
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"title":"Child"}}`

	_, err := tracker.ProcessLine(line)
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestTrackerDetectSpawnRequestInvalidData(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	line := `{"type":"tool_use","tool":"ralph_spawn_feature","tool_input":{"tasks":["only tasks, no title"]}}`

	_, err := tracker.ProcessLine(line)
	if err != ErrInvalidSpawnData {
		t.Errorf("expected ErrInvalidSpawnData, got %v", err)
	}
}

func TestTrackerIgnoresNonToolUseForActions(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	lines := []string{
		`{"type":"assistant","content":"Hello"}`,
		`{"type":"tool_result","result":"done"}`,
		`{"type":"system","content":"init"}`,
	}

	for _, line := range lines {
		tracker.ProcessLine(line)
	}

	actions := feature.GetActions()
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestTrackerEmptyLine(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	req, err := tracker.ProcessLine("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if req != nil {
		t.Error("expected nil request for empty line")
	}
}

func TestTrackerInvalidJSON(t *testing.T) {
	feature := NewRecursiveFeature("test", "Test Feature")
	tracker := NewTracker(feature)

	req, err := tracker.ProcessLine("not json")
	if err != nil {
		t.Errorf("unexpected error for invalid json: %v", err)
	}
	if req != nil {
		t.Error("expected nil request for invalid json")
	}
}

func TestTrackerNilFeature(t *testing.T) {
	tracker := NewTracker(nil)

	req, err := tracker.ProcessLine(`{"type":"tool_use","tool":"Write"}`)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if req != nil {
		t.Error("expected nil request for nil feature")
	}
}

func TestClassifyToolAction(t *testing.T) {
	tests := []struct {
		tool     string
		expected string
	}{
		{"Task", "agent_spawn"},
		{"Agent", "agent_spawn"},
		{"WebFetch", "web_fetch"},
		{"WebSearch", "web_fetch"},
		{"Bash", "command"},
		{"Write", "file_modify"},
		{"Edit", "file_modify"},
		{"Read", "file_read"},
		{"Glob", "search"},
		{"Grep", "search"},
		{"Unknown", ""},
	}

	for _, tc := range tests {
		result := classifyToolAction(tc.tool)
		if result != tc.expected {
			t.Errorf("classifyToolAction(%q) = %q, expected %q", tc.tool, result, tc.expected)
		}
	}
}

func TestExtractToolDetailsLongCommand(t *testing.T) {
	longCmd := "go test -v -race -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/... ./pkg/..."
	input, _ := json.Marshal(map[string]interface{}{"command": longCmd})

	details := extractToolDetails("Bash", input)
	if len(details) > 103 {
		t.Errorf("expected details to be truncated, got length %d", len(details))
	}
}
