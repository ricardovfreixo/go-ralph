package layout

import (
	"strings"
	"testing"
)

func TestNewTaskList(t *testing.T) {
	tl := NewTaskList()
	if tl == nil {
		t.Fatal("NewTaskList returned nil")
	}
	if tl.selected != 0 {
		t.Errorf("expected selected=0, got %d", tl.selected)
	}
	if tl.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0, got %d", tl.scrollOffset)
	}
}

func TestTaskListSetSize(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)
	if tl.width != 80 {
		t.Errorf("expected width=80, got %d", tl.width)
	}
	if tl.height != 20 {
		t.Errorf("expected height=20, got %d", tl.height)
	}
}

func TestTaskListSetItems(t *testing.T) {
	tl := NewTaskList()
	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "pending", Attempts: 0},
		{ID: "2", Title: "Feature 2", Status: "running", Attempts: 1},
		{ID: "3", Title: "Feature 3", Status: "completed", Attempts: 2},
	}
	tl.SetItems(items)
	if len(tl.items) != 3 {
		t.Errorf("expected 3 items, got %d", len(tl.items))
	}
}

func TestTaskListSetSelected(t *testing.T) {
	tl := NewTaskList()
	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "pending"},
		{ID: "2", Title: "Feature 2", Status: "running"},
		{ID: "3", Title: "Feature 3", Status: "completed"},
	}
	tl.SetItems(items)

	tl.SetSelected(1)
	if tl.selected != 1 {
		t.Errorf("expected selected=1, got %d", tl.selected)
	}

	tl.SetSelected(-1)
	if tl.selected != 0 {
		t.Errorf("expected selected=0 for negative, got %d", tl.selected)
	}

	tl.SetSelected(10)
	if tl.selected != 2 {
		t.Errorf("expected selected=2 for overflow, got %d", tl.selected)
	}
}

func TestTaskListNavigation(t *testing.T) {
	tl := NewTaskList()
	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "pending"},
		{ID: "2", Title: "Feature 2", Status: "running"},
		{ID: "3", Title: "Feature 3", Status: "completed"},
	}
	tl.SetItems(items)
	tl.SetSelected(0)

	if !tl.MoveDown() {
		t.Error("MoveDown should return true when not at end")
	}
	if tl.Selected() != 1 {
		t.Errorf("expected selected=1, got %d", tl.Selected())
	}

	if !tl.MoveDown() {
		t.Error("MoveDown should return true when not at end")
	}
	if tl.Selected() != 2 {
		t.Errorf("expected selected=2, got %d", tl.Selected())
	}

	if tl.MoveDown() {
		t.Error("MoveDown should return false at end")
	}
	if tl.Selected() != 2 {
		t.Errorf("expected selected=2, got %d", tl.Selected())
	}

	if !tl.MoveUp() {
		t.Error("MoveUp should return true when not at start")
	}
	if tl.Selected() != 1 {
		t.Errorf("expected selected=1, got %d", tl.Selected())
	}

	tl.SetSelected(0)
	if tl.MoveUp() {
		t.Error("MoveUp should return false at start")
	}
	if tl.Selected() != 0 {
		t.Errorf("expected selected=0, got %d", tl.Selected())
	}
}

func TestTaskListAutoScroll(t *testing.T) {
	tl := NewTaskList()
	items := make([]TaskItem, 20)
	for i := 0; i < 20; i++ {
		items[i] = TaskItem{ID: string(rune('a' + i)), Title: "Feature", Status: "pending"}
	}
	tl.SetItems(items)
	tl.SetSize(80, 5)

	if tl.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0, got %d", tl.scrollOffset)
	}

	tl.SetSelected(10)
	if tl.scrollOffset <= 0 {
		t.Errorf("expected scrollOffset > 0 after selecting item 10, got %d", tl.scrollOffset)
	}

	if tl.selected < tl.scrollOffset || tl.selected >= tl.scrollOffset+5 {
		t.Errorf("selected item %d should be visible (scrollOffset=%d, height=5)", tl.selected, tl.scrollOffset)
	}

	tl.SetSelected(0)
	if tl.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after selecting item 0, got %d", tl.scrollOffset)
	}
}

func TestTaskListRenderStatusIcons(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)

	tests := []struct {
		status   string
		expected string
	}{
		{"pending", "○"},
		{"running", "●"},
		{"completed", "✓"},
		{"failed", "✗"},
		{"stopped", "■"},
	}

	for _, tt := range tests {
		items := []TaskItem{{ID: "1", Title: "Test", Status: tt.status}}
		tl.SetItems(items)
		rendered := tl.Render()
		if !strings.Contains(rendered, tt.expected) {
			t.Errorf("status %q should render icon %q, got: %s", tt.status, tt.expected, rendered)
		}
	}
}

func TestTaskListRenderAttemptCount(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)

	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "pending", Attempts: 1},
		{ID: "2", Title: "Feature 2", Status: "failed", Attempts: 2},
		{ID: "3", Title: "Feature 3", Status: "failed", Attempts: 3},
	}
	tl.SetItems(items)
	rendered := tl.Render()

	if strings.Contains(rendered, "(attempt 1)") {
		t.Error("should not show attempt count for Attempts=1")
	}
	if !strings.Contains(rendered, "(attempt 2)") {
		t.Error("should show (attempt 2) for Attempts=2")
	}
	if !strings.Contains(rendered, "(attempt 3)") {
		t.Error("should show (attempt 3) for Attempts=3")
	}
}

func TestTaskListTruncation(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(30, 20)

	longTitle := "This is a very long feature title that should be truncated"
	items := []TaskItem{{ID: "1", Title: longTitle, Status: "pending"}}
	tl.SetItems(items)
	rendered := tl.Render()

	if strings.Contains(rendered, longTitle) {
		t.Error("long title should be truncated")
	}
	if !strings.Contains(rendered, "...") {
		t.Error("truncated title should end with ...")
	}
}

func TestTaskListTruncateString(t *testing.T) {
	tl := NewTaskList()

	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"tiny", 3, "tiny"},
		{"x", 4, "x"},
		{"toolong", 4, "t..."},
	}

	for _, tt := range tests {
		result := tl.truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTaskListRenderEmpty(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)
	rendered := tl.Render()
	if !strings.Contains(rendered, "No features found") {
		t.Errorf("empty task list should show 'No features found', got: %s", rendered)
	}
}

func TestTaskListSelectedHighlight(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)

	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "pending"},
		{ID: "2", Title: "Feature 2", Status: "pending"},
	}
	tl.SetItems(items)
	tl.SetSelected(0)

	rendered := tl.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines")
	}

	if !strings.Contains(lines[0], "Feature 1") {
		t.Error("first line should contain Feature 1")
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"running", "●"},
		{"completed", "✓"},
		{"failed", "✗"},
		{"stopped", "■"},
		{"pending", "○"},
		{"unknown", "○"},
		{"", "○"},
	}

	for _, tt := range tests {
		result := statusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, result, tt.expected)
		}
	}
}

func TestTaskListScrollBoundaries(t *testing.T) {
	tl := NewTaskList()
	items := make([]TaskItem, 10)
	for i := 0; i < 10; i++ {
		items[i] = TaskItem{ID: string(rune('a' + i)), Title: "Feature", Status: "pending"}
	}
	tl.SetItems(items)
	tl.SetSize(80, 5)

	tl.SetSelected(9)
	if tl.scrollOffset > 5 {
		t.Errorf("scrollOffset should not exceed 5 (10 items - 5 visible), got %d", tl.scrollOffset)
	}

	tl.SetSize(80, 20)
	tl.SetSelected(9)
	if tl.scrollOffset != 0 {
		t.Errorf("scrollOffset should be 0 when all items fit, got %d", tl.scrollOffset)
	}
}

func TestTaskListRenderTokenUsage(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(100, 20)

	items := []TaskItem{
		{ID: "1", Title: "Feature 1", Status: "running", TokenUsage: "1.5k↓ 800↑"},
		{ID: "2", Title: "Feature 2", Status: "completed", TokenUsage: ""},
	}
	tl.SetItems(items)
	rendered := tl.Render()

	if !strings.Contains(rendered, "1.5k↓ 800↑") {
		t.Error("should display token usage when provided")
	}
}
