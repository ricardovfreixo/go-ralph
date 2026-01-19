package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TaskItem struct {
	ID            string
	Title         string
	Status        string
	Attempts      int
	ActionSummary string
	TokenUsage    string
	Cost          string
	BudgetStatus  string
	BudgetAlert   bool
	Model         string // Current model (haiku, sonnet, opus)
	ModelChanged  bool   // Whether model was escalated/de-escalated

	// Hierarchy fields
	ParentID     string   // Empty for root features
	Children     []string // Child feature IDs
	Depth        int      // 0 for root, 1 for child, etc.
	IsExpanded   bool     // Whether children are visible
	IsLastChild  bool     // Whether this is the last child of its parent
	HasChildren  bool     // Whether this feature has children
	ChildCount   int      // Number of direct children
	ChildSummary string   // Aggregated status when collapsed (e.g., "2/3 done")
}

type TaskList struct {
	width        int
	height       int
	items        []TaskItem
	visibleItems []TaskItem // Filtered list based on expand/collapse
	selected     int
	scrollOffset int
	showCost     bool
	expandedMap  map[string]bool // Track which items are expanded
}

func NewTaskList() *TaskList {
	return &TaskList{
		expandedMap: make(map[string]bool),
	}
}

func (t *TaskList) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.ensureSelectedVisible()
}

func (t *TaskList) SetItems(items []TaskItem) {
	t.items = items
	t.rebuildVisibleItems()
	t.ensureSelectedVisible()
}

func (t *TaskList) rebuildVisibleItems() {
	if t.expandedMap == nil {
		t.expandedMap = make(map[string]bool)
	}

	t.visibleItems = make([]TaskItem, 0, len(t.items))
	hiddenParents := make(map[string]bool)

	for _, item := range t.items {
		if item.ParentID != "" && hiddenParents[item.ParentID] {
			hiddenParents[item.ID] = true
			continue
		}

		if item.ParentID != "" {
			parent := t.findItemByID(item.ParentID)
			if parent != nil && !t.expandedMap[parent.ID] {
				hiddenParents[item.ID] = true
				continue
			}
		}

		expanded, exists := t.expandedMap[item.ID]
		if !exists && item.HasChildren {
			t.expandedMap[item.ID] = true
			expanded = true
		}

		visItem := item
		visItem.IsExpanded = expanded
		t.visibleItems = append(t.visibleItems, visItem)

		if !expanded && item.HasChildren {
			hiddenParents[item.ID] = true
		}
	}
}

func (t *TaskList) findItemByID(id string) *TaskItem {
	for i := range t.items {
		if t.items[i].ID == id {
			return &t.items[i]
		}
	}
	return nil
}

func (t *TaskList) ToggleExpand() bool {
	if len(t.visibleItems) == 0 || t.selected >= len(t.visibleItems) {
		return false
	}
	item := t.visibleItems[t.selected]
	if !item.HasChildren {
		return false
	}
	t.expandedMap[item.ID] = !t.expandedMap[item.ID]
	t.rebuildVisibleItems()
	return true
}

func (t *TaskList) Expand() bool {
	if len(t.visibleItems) == 0 || t.selected >= len(t.visibleItems) {
		return false
	}
	item := t.visibleItems[t.selected]
	if !item.HasChildren || t.expandedMap[item.ID] {
		return false
	}
	t.expandedMap[item.ID] = true
	t.rebuildVisibleItems()
	return true
}

func (t *TaskList) Collapse() bool {
	if len(t.visibleItems) == 0 || t.selected >= len(t.visibleItems) {
		return false
	}
	item := t.visibleItems[t.selected]
	if !item.HasChildren || !t.expandedMap[item.ID] {
		return false
	}
	t.expandedMap[item.ID] = false
	t.rebuildVisibleItems()
	return true
}

func (t *TaskList) IsExpanded(id string) bool {
	if t.expandedMap == nil {
		return true
	}
	expanded, exists := t.expandedMap[id]
	if !exists {
		return true
	}
	return expanded
}

func (t *TaskList) ExpandAll() {
	for i := range t.items {
		if t.items[i].HasChildren {
			t.expandedMap[t.items[i].ID] = true
		}
	}
	t.rebuildVisibleItems()
}

func (t *TaskList) CollapseAll() {
	for i := range t.items {
		if t.items[i].HasChildren {
			t.expandedMap[t.items[i].ID] = false
		}
	}
	t.rebuildVisibleItems()
}

func (t *TaskList) SelectedItem() *TaskItem {
	if len(t.visibleItems) == 0 || t.selected >= len(t.visibleItems) {
		return nil
	}
	return &t.visibleItems[t.selected]
}

func (t *TaskList) SetSelected(index int) {
	if index < 0 {
		index = 0
	}
	if len(t.visibleItems) > 0 && index >= len(t.visibleItems) {
		index = len(t.visibleItems) - 1
	}
	t.selected = index
	t.ensureSelectedVisible()
}

func (t *TaskList) Selected() int {
	return t.selected
}

func (t *TaskList) VisibleCount() int {
	return len(t.visibleItems)
}

func (t *TaskList) TotalCount() int {
	return len(t.items)
}

func (t *TaskList) MoveUp() bool {
	if t.selected > 0 {
		t.selected--
		t.ensureSelectedVisible()
		return true
	}
	return false
}

func (t *TaskList) MoveDown() bool {
	if t.selected < len(t.visibleItems)-1 {
		t.selected++
		t.ensureSelectedVisible()
		return true
	}
	return false
}

func (t *TaskList) SetShowCost(show bool) {
	t.showCost = show
}

func (t *TaskList) ToggleCost() bool {
	t.showCost = !t.showCost
	return t.showCost
}

func (t *TaskList) ShowCost() bool {
	return t.showCost
}

func (t *TaskList) ensureSelectedVisible() {
	if t.height <= 0 || len(t.visibleItems) == 0 {
		return
	}

	visibleLines := t.height
	if visibleLines < 1 {
		visibleLines = 1
	}

	if t.selected < t.scrollOffset {
		t.scrollOffset = t.selected
	}

	if t.selected >= t.scrollOffset+visibleLines {
		t.scrollOffset = t.selected - visibleLines + 1
	}

	maxOffset := len(t.visibleItems) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.scrollOffset > maxOffset {
		t.scrollOffset = maxOffset
	}
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}

func (t *TaskList) Render() string {
	if len(t.visibleItems) == 0 {
		return "No features found."
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSelectedFg).
		Background(colorSelected)

	normalStyle := lipgloss.NewStyle().
		Foreground(colorNormal)

	dimStyle := lipgloss.NewStyle().
		Foreground(colorSubtle)

	actionStyle := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Italic(true)

	treeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	statusStyle := func(status string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(StatusColor(status))
	}

	visibleLines := t.height
	if visibleLines < 1 {
		visibleLines = 1
	}

	startIdx := t.scrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(t.visibleItems) {
		endIdx = len(t.visibleItems)
	}

	maxWidth := t.width - 4
	if maxWidth < 10 {
		maxWidth = 10
	}

	usageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	costStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))

	budgetAlertStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	childSummaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

	modelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141"))

	modelChangedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var lines []string
	for i := startIdx; i < endIdx; i++ {
		item := t.visibleItems[i]
		icon := statusIcon(item.Status)

		// Build tree prefix for nested items
		treePrefix := t.buildTreePrefix(item)

		// Add expand/collapse indicator for items with children
		expandIndicator := ""
		if item.HasChildren {
			if item.IsExpanded {
				expandIndicator = "▼ "
			} else {
				expandIndicator = "▶ "
			}
		}

		attemptStr := ""
		if item.Attempts > 1 {
			attemptStr = fmt.Sprintf(" (attempt %d)", item.Attempts)
		}

		actionStr := ""
		if item.ActionSummary != "" {
			actionStr = " [" + item.ActionSummary + "]"
		}

		// Show child summary when collapsed
		childSummaryStr := ""
		if item.HasChildren && !item.IsExpanded && item.ChildSummary != "" {
			childSummaryStr = " " + item.ChildSummary
		}

		// Show model indicator when running or changed
		modelStr := ""
		var modelStyleToUse lipgloss.Style
		if item.Model != "" && (item.Status == "running" || item.ModelChanged) {
			modelStr = " [" + modelIcon(item.Model) + "]"
			if item.ModelChanged {
				modelStyleToUse = modelChangedStyle
			} else {
				modelStyleToUse = modelStyle
			}
		}

		usageOrCostStr := ""
		var usageOrCostStyle lipgloss.Style
		if item.BudgetStatus != "" {
			usageOrCostStr = " " + item.BudgetStatus
			if item.BudgetAlert {
				usageOrCostStyle = budgetAlertStyle
			} else {
				usageOrCostStyle = usageStyle
			}
		} else if t.showCost && item.Cost != "" {
			usageOrCostStr = " " + item.Cost
			usageOrCostStyle = costStyle
		} else if item.TokenUsage != "" {
			usageOrCostStr = " " + item.TokenUsage
			usageOrCostStyle = usageStyle
		}

		treePrefixWidth := lipgloss.Width(treePrefix) + lipgloss.Width(expandIndicator)
		titleMaxLen := maxWidth - 5 - treePrefixWidth - len(attemptStr) - len(actionStr) - lipgloss.Width(childSummaryStr) - len(modelStr) - lipgloss.Width(usageOrCostStr)
		displayTitle := t.truncateString(item.Title, titleMaxLen)

		line := fmt.Sprintf(" %s%s%s  %s%s%s%s%s%s",
			treeStyle.Render(treePrefix),
			treeStyle.Render(expandIndicator),
			statusStyle(item.Status).Render(icon),
			displayTitle,
			dimStyle.Render(attemptStr),
			actionStyle.Render(actionStr),
			childSummaryStyle.Render(childSummaryStr),
			modelStyleToUse.Render(modelStr),
			usageOrCostStyle.Render(usageOrCostStr))

		if i == t.selected {
			line = t.padToWidth(line, maxWidth)
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func (t *TaskList) buildTreePrefix(item TaskItem) string {
	if item.Depth == 0 {
		return ""
	}

	// Build the prefix from the ancestors
	prefix := ""
	for d := 1; d < item.Depth; d++ {
		// Find ancestor at this depth to check if it's a last child
		ancestor := t.findAncestorAtDepth(item, d)
		if ancestor != nil && ancestor.IsLastChild {
			prefix += "    "
		} else {
			prefix += "│   "
		}
	}

	// Add the branch for this item
	if item.IsLastChild {
		prefix += "└── "
	} else {
		prefix += "├── "
	}

	return prefix
}

func (t *TaskList) findAncestorAtDepth(item TaskItem, depth int) *TaskItem {
	if depth >= item.Depth || depth < 1 {
		return nil
	}

	// Walk up the tree to find the ancestor at the given depth
	current := &item
	for current.Depth > depth {
		parent := t.findItemByID(current.ParentID)
		if parent == nil {
			return nil
		}
		current = parent
	}
	return current
}

// CalculateChildSummary generates an aggregated status string for a parent feature.
// Returns a string like "(2/3 done)" showing completed/total child count.
func CalculateChildSummary(items []TaskItem, parentID string) string {
	total := 0
	completed := 0
	running := 0

	for _, item := range items {
		if item.ParentID == parentID {
			total++
			if item.Status == "completed" {
				completed++
			} else if item.Status == "running" {
				running++
			}
		}
	}

	if total == 0 {
		return ""
	}

	if running > 0 {
		return fmt.Sprintf("(%d/%d done, %d running)", completed, total, running)
	}
	return fmt.Sprintf("(%d/%d done)", completed, total)
}

func (t *TaskList) truncateString(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (t *TaskList) padToWidth(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return "●"
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "stopped":
		return "■"
	default:
		return "○"
	}
}

func modelIcon(model string) string {
	switch model {
	case "haiku":
		return "H"
	case "sonnet":
		return "S"
	case "opus":
		return "O"
	default:
		return model
	}
}
