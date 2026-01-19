package usage

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	u := New()
	if u == nil {
		t.Fatal("New() returned nil")
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 {
		t.Error("New usage should have zero tokens")
	}
}

func TestParseLineTopLevelUsage(t *testing.T) {
	u := New()
	line := `{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50},"cost_usd":0.01}`

	if !u.ParseLine(line) {
		t.Error("ParseLine should return true for valid usage")
	}

	if u.InputTokens != 100 {
		t.Errorf("expected InputTokens 100, got %d", u.InputTokens)
	}
	if u.OutputTokens != 50 {
		t.Errorf("expected OutputTokens 50, got %d", u.OutputTokens)
	}
	if u.TotalTokens != 150 {
		t.Errorf("expected TotalTokens 150, got %d", u.TotalTokens)
	}
	if u.CostUSD != 0.01 {
		t.Errorf("expected CostUSD 0.01, got %f", u.CostUSD)
	}
}

func TestParseLineNestedUsage(t *testing.T) {
	u := New()

	msg := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":  200,
			"output_tokens": 100,
		},
	}
	msgBytes, _ := json.Marshal(msg)
	line := `{"type":"assistant","message":` + string(msgBytes) + `}`

	if !u.ParseLine(line) {
		t.Error("ParseLine should return true for nested usage")
	}

	if u.InputTokens != 200 {
		t.Errorf("expected InputTokens 200, got %d", u.InputTokens)
	}
	if u.OutputTokens != 100 {
		t.Errorf("expected OutputTokens 100, got %d", u.OutputTokens)
	}
}

func TestParseLineCacheTokens(t *testing.T) {
	u := New()
	line := `{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":30,"cache_creation_input_tokens":20}}`

	u.ParseLine(line)

	if u.CacheReadTokens != 30 {
		t.Errorf("expected CacheReadTokens 30, got %d", u.CacheReadTokens)
	}
	if u.CacheWriteTokens != 20 {
		t.Errorf("expected CacheWriteTokens 20, got %d", u.CacheWriteTokens)
	}
}

func TestParseLineCostOnlyResult(t *testing.T) {
	u := New()
	line := `{"type":"result","subtype":"success","cost_usd":0.05}`

	if !u.ParseLine(line) {
		t.Error("ParseLine should return true for cost-only result")
	}

	if u.CostUSD != 0.05 {
		t.Errorf("expected CostUSD 0.05, got %f", u.CostUSD)
	}
}

func TestParseLineEmptyLine(t *testing.T) {
	u := New()
	if u.ParseLine("") {
		t.Error("ParseLine should return false for empty line")
	}
}

func TestParseLineInvalidJSON(t *testing.T) {
	u := New()
	if u.ParseLine("not json") {
		t.Error("ParseLine should return false for invalid JSON")
	}
}

func TestParseLineNoUsage(t *testing.T) {
	u := New()
	line := `{"type":"tool_use","tool":"Bash"}`

	if u.ParseLine(line) {
		t.Error("ParseLine should return false for message without usage")
	}
}

func TestAccumulation(t *testing.T) {
	u := New()

	lines := []string{
		`{"type":"assistant","usage":{"input_tokens":100,"output_tokens":50},"cost_usd":0.01}`,
		`{"type":"assistant","usage":{"input_tokens":200,"output_tokens":100},"cost_usd":0.02}`,
		`{"type":"assistant","usage":{"input_tokens":300,"output_tokens":150},"cost_usd":0.03}`,
	}

	for _, line := range lines {
		u.ParseLine(line)
	}

	if u.InputTokens != 600 {
		t.Errorf("expected accumulated InputTokens 600, got %d", u.InputTokens)
	}
	if u.OutputTokens != 300 {
		t.Errorf("expected accumulated OutputTokens 300, got %d", u.OutputTokens)
	}
	if u.TotalTokens != 900 {
		t.Errorf("expected accumulated TotalTokens 900, got %d", u.TotalTokens)
	}
	if u.CostUSD != 0.06 {
		t.Errorf("expected accumulated CostUSD 0.06, got %f", u.CostUSD)
	}
}

func TestAdd(t *testing.T) {
	u1 := New()
	u1.InputTokens = 100
	u1.OutputTokens = 50
	u1.TotalTokens = 150
	u1.CostUSD = 0.01

	u2 := New()
	u2.InputTokens = 200
	u2.OutputTokens = 100
	u2.TotalTokens = 300
	u2.CostUSD = 0.02

	u1.Add(u2)

	if u1.InputTokens != 300 {
		t.Errorf("expected InputTokens 300, got %d", u1.InputTokens)
	}
	if u1.OutputTokens != 150 {
		t.Errorf("expected OutputTokens 150, got %d", u1.OutputTokens)
	}
	if u1.TotalTokens != 450 {
		t.Errorf("expected TotalTokens 450, got %d", u1.TotalTokens)
	}
}

func TestAddNil(t *testing.T) {
	u := New()
	u.InputTokens = 100
	u.Add(nil)

	if u.InputTokens != 100 {
		t.Error("Add(nil) should not modify usage")
	}
}

func TestSnapshot(t *testing.T) {
	u := New()
	u.InputTokens = 100
	u.OutputTokens = 50
	u.TotalTokens = 150
	u.CostUSD = 0.01

	snap := u.Snapshot()

	if snap.InputTokens != u.InputTokens {
		t.Error("Snapshot InputTokens mismatch")
	}
	if snap.OutputTokens != u.OutputTokens {
		t.Error("Snapshot OutputTokens mismatch")
	}

	u.InputTokens = 999
	if snap.InputTokens == u.InputTokens {
		t.Error("Snapshot should be independent copy")
	}
}

func TestReset(t *testing.T) {
	u := New()
	u.InputTokens = 100
	u.OutputTokens = 50
	u.TotalTokens = 150
	u.CostUSD = 0.01

	u.Reset()

	if u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 || u.CostUSD != 0 {
		t.Error("Reset should zero all fields")
	}
}

func TestIsEmpty(t *testing.T) {
	u := New()
	if !u.IsEmpty() {
		t.Error("New usage should be empty")
	}

	u.InputTokens = 100
	u.TotalTokens = 100
	if u.IsEmpty() {
		t.Error("Usage with tokens should not be empty")
	}

	u.Reset()
	u.CostUSD = 0.01
	if u.IsEmpty() {
		t.Error("Usage with cost should not be empty")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{10000, "10.0k"},
		{100000, "100.0k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tc := range tests {
		result := FormatTokens(tc.n)
		if result != tc.expected {
			t.Errorf("FormatTokens(%d) = %q, expected %q", tc.n, result, tc.expected)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0, ""},
		{0.001, "$0.0010"},
		{0.009, "$0.0090"},
		{0.01, "$0.01"},
		{0.05, "$0.05"},
		{1.23, "$1.23"},
	}

	for _, tc := range tests {
		result := FormatCost(tc.cost)
		if result != tc.expected {
			t.Errorf("FormatCost(%f) = %q, expected %q", tc.cost, result, tc.expected)
		}
	}
}

func TestCompact(t *testing.T) {
	u := New()
	if u.Compact() != "" {
		t.Error("Compact on empty usage should return empty string")
	}

	u.InputTokens = 1500
	u.OutputTokens = 800
	u.TotalTokens = 2300

	compact := u.Compact()
	if compact != "1.5k↓ 800↑" {
		t.Errorf("unexpected Compact output: %q", compact)
	}
}

func TestDetailed(t *testing.T) {
	u := New()
	if u.Detailed() != "No usage recorded" {
		t.Error("Detailed on empty usage should say no usage")
	}

	u.InputTokens = 1500
	u.OutputTokens = 800
	u.TotalTokens = 2300

	detailed := u.Detailed()
	if detailed != "Input: 1.5k  Output: 800  Total: 2.3k" {
		t.Errorf("unexpected Detailed output: %q", detailed)
	}

	u.CacheReadTokens = 500
	u.CacheWriteTokens = 200
	u.CostUSD = 0.05

	detailed = u.Detailed()
	expected := "Input: 1.5k  Output: 800  Total: 2.3k\nCache: 500 read, 200 write\nCost: $0.05"
	if detailed != expected {
		t.Errorf("unexpected Detailed output:\ngot:  %q\nwant: %q", detailed, expected)
	}
}

func TestConcurrentAccess(t *testing.T) {
	u := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u.ParseLine(`{"type":"assistant","usage":{"input_tokens":10,"output_tokens":5},"cost_usd":0.001}`)
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = u.Snapshot()
		}()
	}

	wg.Wait()

	if u.InputTokens != 1000 {
		t.Errorf("expected InputTokens 1000, got %d", u.InputTokens)
	}
}

func TestRealWorldStreamJSON(t *testing.T) {
	u := New()

	// Simulating real Claude Code stream-json output patterns
	lines := []string{
		// Initial assistant message with nested usage
		`{"type":"assistant","message":{"model":"claude-opus-4-5-20251101","id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"usage":{"input_tokens":1500,"output_tokens":200,"cache_read_input_tokens":500}}}`,
		// Tool use - no usage
		`{"type":"tool_use","tool":"Bash","tool_input":{"command":"ls"}}`,
		// Tool result - no usage
		`{"type":"tool_result","result":"file1.go\nfile2.go"}`,
		// Another assistant with more output
		`{"type":"assistant","usage":{"input_tokens":2000,"output_tokens":500,"cache_read_input_tokens":1000}}`,
		// Final result with cost
		`{"type":"result","subtype":"success","cost_usd":0.15}`,
	}

	for _, line := range lines {
		u.ParseLine(line)
	}

	if u.InputTokens != 3500 {
		t.Errorf("expected InputTokens 3500, got %d", u.InputTokens)
	}
	if u.OutputTokens != 700 {
		t.Errorf("expected OutputTokens 700, got %d", u.OutputTokens)
	}
	if u.CacheReadTokens != 1500 {
		t.Errorf("expected CacheReadTokens 1500, got %d", u.CacheReadTokens)
	}
	if u.CostUSD != 0.15 {
		t.Errorf("expected CostUSD 0.15, got %f", u.CostUSD)
	}
}
