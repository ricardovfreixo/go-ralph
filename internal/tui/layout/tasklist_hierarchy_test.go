package layout

import (
	"strings"
	"testing"
)

func TestTaskListWithHierarchy(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root Feature 1", Status: "running", Depth: 0, HasChildren: true, Children: []string{"01-01", "01-02"}, ChildCount: 2},
		{ID: "01-01", Title: "Child 1.1", Status: "completed", Depth: 1, ParentID: "01", IsLastChild: false},
		{ID: "01-02", Title: "Child 1.2", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
		{ID: "02", Title: "Root Feature 2", Status: "pending", Depth: 0, IsLastChild: true},
	}
	tl.SetItems(items)

	if tl.TotalCount() != 4 {
		t.Errorf("Expected total count 4, got %d", tl.TotalCount())
	}

	// With default expansion, all items should be visible
	if tl.VisibleCount() != 4 {
		t.Errorf("Expected visible count 4, got %d", tl.VisibleCount())
	}
}

func TestTaskListCollapse(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root Feature 1", Status: "running", Depth: 0, HasChildren: true, Children: []string{"01-01", "01-02"}, ChildCount: 2},
		{ID: "01-01", Title: "Child 1.1", Status: "completed", Depth: 1, ParentID: "01", IsLastChild: false},
		{ID: "01-02", Title: "Child 1.2", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
		{ID: "02", Title: "Root Feature 2", Status: "pending", Depth: 0, IsLastChild: true},
	}
	tl.SetItems(items)

	// Select the first item (parent with children)
	tl.SetSelected(0)

	// Initially expanded
	if !tl.IsExpanded("01") {
		t.Error("Expected item 01 to be expanded by default")
	}

	// Collapse it
	toggled := tl.ToggleExpand()
	if !toggled {
		t.Error("Expected ToggleExpand to return true")
	}

	// Should now be collapsed
	if tl.IsExpanded("01") {
		t.Error("Expected item 01 to be collapsed after toggle")
	}

	// Children should be hidden
	if tl.VisibleCount() != 2 {
		t.Errorf("Expected visible count 2 after collapse, got %d", tl.VisibleCount())
	}
}

func TestTaskListExpand(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root Feature 1", Status: "running", Depth: 0, HasChildren: true, Children: []string{"01-01"}, ChildCount: 1},
		{ID: "01-01", Title: "Child 1.1", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
	}
	tl.SetItems(items)

	// Collapse first
	tl.SetSelected(0)
	tl.Collapse()
	if tl.IsExpanded("01") {
		t.Error("Expected item 01 to be collapsed")
	}

	// Now expand
	tl.Expand()
	if !tl.IsExpanded("01") {
		t.Error("Expected item 01 to be expanded")
	}
	if tl.VisibleCount() != 2 {
		t.Errorf("Expected visible count 2 after expand, got %d", tl.VisibleCount())
	}
}

func TestTaskListExpandCollapseAll(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root 1", Status: "pending", Depth: 0, HasChildren: true, Children: []string{"01-01"}, ChildCount: 1},
		{ID: "01-01", Title: "Child 1.1", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
		{ID: "02", Title: "Root 2", Status: "pending", Depth: 0, HasChildren: true, Children: []string{"02-01"}, ChildCount: 1},
		{ID: "02-01", Title: "Child 2.1", Status: "pending", Depth: 1, ParentID: "02", IsLastChild: true},
	}
	tl.SetItems(items)

	// Collapse all
	tl.CollapseAll()
	if tl.VisibleCount() != 2 {
		t.Errorf("Expected visible count 2 after CollapseAll, got %d", tl.VisibleCount())
	}

	// Expand all
	tl.ExpandAll()
	if tl.VisibleCount() != 4 {
		t.Errorf("Expected visible count 4 after ExpandAll, got %d", tl.VisibleCount())
	}
}

func TestTaskListSelectedItem(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Feature 1", Status: "pending"},
		{ID: "02", Title: "Feature 2", Status: "running"},
	}
	tl.SetItems(items)

	tl.SetSelected(1)
	item := tl.SelectedItem()
	if item == nil {
		t.Fatal("Expected non-nil selected item")
	}
	if item.ID != "02" {
		t.Errorf("Expected selected item ID 02, got %s", item.ID)
	}
}

func TestTaskListRenderTreeLines(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Parent", Status: "running", Depth: 0, HasChildren: true, IsExpanded: true, Children: []string{"01-01", "01-02"}, ChildCount: 2},
		{ID: "01-01", Title: "Child 1", Status: "completed", Depth: 1, ParentID: "01", IsLastChild: false},
		{ID: "01-02", Title: "Child 2", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
	}
	tl.SetItems(items)

	output := tl.Render()

	// Check for tree characters
	if !strings.Contains(output, "├──") {
		t.Error("Expected tree branch ├── in output for non-last child")
	}
	if !strings.Contains(output, "└──") {
		t.Error("Expected tree branch └── in output for last child")
	}
}

func TestTaskListRenderExpandCollapseIndicators(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Expanded Parent", Status: "pending", Depth: 0, HasChildren: true, Children: []string{"01-01"}, ChildCount: 1},
		{ID: "01-01", Title: "Child", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
	}
	tl.SetItems(items)

	// Expanded state
	output := tl.Render()
	if !strings.Contains(output, "▼") {
		t.Error("Expected expanded indicator ▼ in output")
	}

	// Collapse and check
	tl.SetSelected(0)
	tl.Collapse()
	output = tl.Render()
	if !strings.Contains(output, "▶") {
		t.Error("Expected collapsed indicator ▶ in output")
	}
}

func TestTaskListRenderChildSummary(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Parent", Status: "running", Depth: 0, HasChildren: true, Children: []string{"01-01", "01-02"}, ChildCount: 2, ChildSummary: "(1/2 done)"},
		{ID: "01-01", Title: "Child 1", Status: "completed", Depth: 1, ParentID: "01"},
		{ID: "01-02", Title: "Child 2", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
	}
	tl.SetItems(items)

	// Collapse to show summary
	tl.SetSelected(0)
	tl.Collapse()

	output := tl.Render()
	if !strings.Contains(output, "(1/2 done)") {
		t.Error("Expected child summary (1/2 done) in output when collapsed")
	}
}

func TestCalculateChildSummary(t *testing.T) {
	tests := []struct {
		name     string
		items    []TaskItem
		parentID string
		expected string
	}{
		{
			name: "all completed",
			items: []TaskItem{
				{ID: "01", ParentID: ""},
				{ID: "01-01", ParentID: "01", Status: "completed"},
				{ID: "01-02", ParentID: "01", Status: "completed"},
			},
			parentID: "01",
			expected: "(2/2 done)",
		},
		{
			name: "some completed",
			items: []TaskItem{
				{ID: "01", ParentID: ""},
				{ID: "01-01", ParentID: "01", Status: "completed"},
				{ID: "01-02", ParentID: "01", Status: "pending"},
				{ID: "01-03", ParentID: "01", Status: "failed"},
			},
			parentID: "01",
			expected: "(1/3 done)",
		},
		{
			name: "with running",
			items: []TaskItem{
				{ID: "01", ParentID: ""},
				{ID: "01-01", ParentID: "01", Status: "completed"},
				{ID: "01-02", ParentID: "01", Status: "running"},
			},
			parentID: "01",
			expected: "(1/2 done, 1 running)",
		},
		{
			name:     "no children",
			items:    []TaskItem{{ID: "01", ParentID: ""}},
			parentID: "01",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateChildSummary(tt.items, tt.parentID)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTaskListNavigationWithHierarchy(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root 1", Status: "pending", Depth: 0, HasChildren: true, Children: []string{"01-01"}, ChildCount: 1},
		{ID: "01-01", Title: "Child 1", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
		{ID: "02", Title: "Root 2", Status: "pending", Depth: 0, IsLastChild: true},
	}
	tl.SetItems(items)

	// Navigate down
	tl.SetSelected(0)
	if !tl.MoveDown() {
		t.Error("Expected MoveDown to return true")
	}
	if tl.Selected() != 1 {
		t.Errorf("Expected selection 1, got %d", tl.Selected())
	}

	// Navigate up
	if !tl.MoveUp() {
		t.Error("Expected MoveUp to return true")
	}
	if tl.Selected() != 0 {
		t.Errorf("Expected selection 0, got %d", tl.Selected())
	}

	// Can't go above 0
	if tl.MoveUp() {
		t.Error("Expected MoveUp to return false at top")
	}

	// Move to end
	tl.SetSelected(2)
	// Can't go below last
	if tl.MoveDown() {
		t.Error("Expected MoveDown to return false at bottom")
	}
}

func TestTaskListNavigationAfterCollapse(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root 1", Status: "pending", Depth: 0, HasChildren: true, Children: []string{"01-01", "01-02"}, ChildCount: 2},
		{ID: "01-01", Title: "Child 1.1", Status: "pending", Depth: 1, ParentID: "01"},
		{ID: "01-02", Title: "Child 1.2", Status: "pending", Depth: 1, ParentID: "01", IsLastChild: true},
		{ID: "02", Title: "Root 2", Status: "pending", Depth: 0, IsLastChild: true},
	}
	tl.SetItems(items)

	// Select a child
	tl.SetSelected(1)

	// Collapse parent (need to select it first)
	tl.SetSelected(0)
	tl.Collapse()

	// Visible count should now be 2
	if tl.VisibleCount() != 2 {
		t.Errorf("Expected visible count 2, got %d", tl.VisibleCount())
	}

	// Navigate should work within visible items
	tl.SetSelected(0)
	tl.MoveDown()
	item := tl.SelectedItem()
	if item == nil || item.ID != "02" {
		t.Error("Expected to navigate to Root 2 after collapse")
	}
}

func TestTaskListToggleExpandNoChildren(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Leaf Feature", Status: "pending", Depth: 0, HasChildren: false},
	}
	tl.SetItems(items)

	tl.SetSelected(0)
	if tl.ToggleExpand() {
		t.Error("Expected ToggleExpand to return false for item without children")
	}
}

func TestBuildTreePrefix(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 10)

	items := []TaskItem{
		{ID: "01", Title: "Root", Depth: 0, HasChildren: true, Children: []string{"01-01"}},
		{ID: "01-01", Title: "Child", Depth: 1, ParentID: "01", IsLastChild: true},
	}
	tl.SetItems(items)

	// Root should have no prefix
	rootPrefix := tl.buildTreePrefix(items[0])
	if rootPrefix != "" {
		t.Errorf("Expected empty prefix for root, got %q", rootPrefix)
	}

	// Child should have prefix
	childPrefix := tl.buildTreePrefix(items[1])
	if childPrefix != "└── " {
		t.Errorf("Expected '└── ' prefix for last child, got %q", childPrefix)
	}
}

func TestTaskListDeepHierarchy(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(80, 20)

	items := []TaskItem{
		{ID: "01", Title: "Root", Depth: 0, HasChildren: true, Children: []string{"01-01"}},
		{ID: "01-01", Title: "Level 1", Depth: 1, ParentID: "01", HasChildren: true, Children: []string{"01-01-01"}, IsLastChild: true},
		{ID: "01-01-01", Title: "Level 2", Depth: 2, ParentID: "01-01", IsLastChild: true},
	}
	tl.SetItems(items)

	if tl.VisibleCount() != 3 {
		t.Errorf("Expected 3 visible items, got %d", tl.VisibleCount())
	}

	output := tl.Render()
	// Should contain nested tree structure
	if !strings.Contains(output, "Level 1") {
		t.Error("Expected 'Level 1' in output")
	}
	if !strings.Contains(output, "Level 2") {
		t.Error("Expected 'Level 2' in output")
	}
}
