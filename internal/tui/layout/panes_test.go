package layout

import (
	"strings"
	"testing"
)

func TestPaneFocusConstants(t *testing.T) {
	if FocusLeft != 0 {
		t.Error("FocusLeft should be 0")
	}
	if FocusRight != 1 {
		t.Error("FocusRight should be 1")
	}
}

func TestSplitPaneSetFocus(t *testing.T) {
	sp := NewSplitPane()

	if sp.Focus() != FocusLeft {
		t.Error("Default focus should be left pane")
	}

	sp.SetFocus(FocusRight)
	if sp.Focus() != FocusRight {
		t.Error("Focus should be right pane after SetFocus(FocusRight)")
	}

	sp.SetFocus(FocusLeft)
	if sp.Focus() != FocusLeft {
		t.Error("Focus should be left pane after SetFocus(FocusLeft)")
	}
}

func TestSplitPaneFocusedTitleStyle(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)
	sp.SetFocus(FocusLeft)

	leftContent := "Left content"
	rightContent := "Right content"
	result := sp.Render(leftContent, rightContent)

	if result == "" {
		t.Error("SplitPane should render with focus")
	}
	if !strings.Contains(result, "TASKS") {
		t.Error("Should contain left pane title")
	}
	if !strings.Contains(result, "ACTIVITY") {
		t.Error("Should contain right pane title")
	}
}

func TestSplitPaneNew(t *testing.T) {
	sp := NewSplitPane()

	if sp.leftTitle != "TASKS" {
		t.Errorf("Expected left title 'TASKS', got '%s'", sp.leftTitle)
	}
	if sp.rightTitle != "ACTIVITY" {
		t.Errorf("Expected right title 'ACTIVITY', got '%s'", sp.rightTitle)
	}
}

func TestSplitPaneSetSize(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(100, 30)

	if sp.Width() != 100 {
		t.Errorf("Expected width 100, got %d", sp.Width())
	}
	if sp.Height() != 30 {
		t.Errorf("Expected height 30, got %d", sp.Height())
	}
}

func TestSplitPaneSetTitles(t *testing.T) {
	sp := NewSplitPane()
	sp.SetTitles("LEFT", "RIGHT")

	if sp.leftTitle != "LEFT" {
		t.Errorf("Expected left title 'LEFT', got '%s'", sp.leftTitle)
	}
	if sp.rightTitle != "RIGHT" {
		t.Errorf("Expected right title 'RIGHT', got '%s'", sp.rightTitle)
	}
}

func TestSplitPaneWidthCalculation(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(101, 30)

	leftWidth := sp.LeftPaneWidth()
	rightWidth := sp.RightPaneWidth()

	totalUsed := leftWidth + rightWidth + DividerWidth
	if totalUsed != 101 {
		t.Errorf("Expected total width 101, got %d (left=%d, right=%d, divider=%d)",
			totalUsed, leftWidth, rightWidth, DividerWidth)
	}

	if leftWidth < MinPaneWidth {
		t.Errorf("Left pane width %d is below minimum %d", leftWidth, MinPaneWidth)
	}
	if rightWidth < MinPaneWidth {
		t.Errorf("Right pane width %d is below minimum %d", rightWidth, MinPaneWidth)
	}
}

func TestSplitPaneWidthCalculationEven(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(100, 30)

	leftWidth := sp.LeftPaneWidth()
	rightWidth := sp.RightPaneWidth()

	totalUsed := leftWidth + rightWidth + DividerWidth
	if totalUsed != 100 {
		t.Errorf("Expected total width 100, got %d", totalUsed)
	}
}

func TestSplitPaneContentHeight(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(100, 20)

	contentHeight := sp.ContentHeight()
	expectedHeight := 20 - PaneHeaderHeight

	if contentHeight != expectedHeight {
		t.Errorf("Expected content height %d, got %d", expectedHeight, contentHeight)
	}
}

func TestSplitPaneContentHeightMinimum(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(100, 1)

	contentHeight := sp.ContentHeight()
	if contentHeight < 1 {
		t.Error("Content height should be at least 1")
	}
}

func TestSplitPaneRenderEmpty(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(0, 0)

	result := sp.Render("", "")
	if result != "" {
		t.Error("Expected empty string for zero dimensions")
	}
}

func TestSplitPaneRenderContainsHeaders(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)

	result := sp.Render("content", "activity")

	if !strings.Contains(result, "TASKS") {
		t.Error("Render should contain left pane header 'TASKS'")
	}
	if !strings.Contains(result, "ACTIVITY") {
		t.Error("Render should contain right pane header 'ACTIVITY'")
	}
}

func TestSplitPaneRenderContainsDivider(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)

	result := sp.Render("left", "right")

	if !strings.Contains(result, "│") {
		t.Error("Render should contain vertical divider character")
	}
}

func TestSplitPaneRenderContainsContent(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)

	result := sp.Render("left content here", "right content here")

	if !strings.Contains(result, "left content") {
		t.Error("Render should contain left pane content")
	}
	if !strings.Contains(result, "right content") {
		t.Error("Render should contain right pane content")
	}
}

func TestSplitPaneRenderMultilineContent(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)

	leftContent := "Line 1\nLine 2\nLine 3"
	rightContent := "Activity 1\nActivity 2"

	result := sp.Render(leftContent, rightContent)

	if !strings.Contains(result, "Line 1") {
		t.Error("Render should contain first line of left content")
	}
	if !strings.Contains(result, "Line 3") {
		t.Error("Render should contain third line of left content")
	}
	if !strings.Contains(result, "Activity 1") {
		t.Error("Render should contain first activity")
	}
}

func TestSplitPaneRenderTruncatesLongContent(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(40, 10)

	longLine := strings.Repeat("x", 100)
	result := sp.Render(longLine, "short")

	if result == "" {
		t.Error("Should render content even with long lines")
	}
	if !strings.Contains(result, "...") {
		t.Error("Long content should be truncated with ellipsis")
	}
}

func TestSplitPaneRenderTruncatesExcessLines(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 5)

	manyLines := strings.Repeat("line\n", 50)
	result := sp.Render(manyLines, "")

	lineCount := strings.Count(result, "\n") + 1
	expectedMaxLines := 5 + 2

	if lineCount > expectedMaxLines {
		t.Errorf("Expected at most %d lines, got %d", expectedMaxLines, lineCount)
	}
}

func TestSplitPaneResizeProportionally(t *testing.T) {
	sp := NewSplitPane()

	sp.SetSize(80, 20)
	leftWidth80 := sp.LeftPaneWidth()
	rightWidth80 := sp.RightPaneWidth()

	sp.SetSize(120, 20)
	leftWidth120 := sp.LeftPaneWidth()
	rightWidth120 := sp.RightPaneWidth()

	if leftWidth120 <= leftWidth80 {
		t.Error("Left pane should grow when terminal width increases")
	}
	if rightWidth120 <= rightWidth80 {
		t.Error("Right pane should grow when terminal width increases")
	}

	ratio80 := float64(leftWidth80) / float64(rightWidth80)
	ratio120 := float64(leftWidth120) / float64(rightWidth120)

	diff := ratio80 - ratio120
	if diff < -0.1 || diff > 0.1 {
		t.Errorf("Pane ratio should remain approximately 50/50: 80w ratio=%f, 120w ratio=%f", ratio80, ratio120)
	}
}

func TestSplitPaneCustomTitles(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)
	sp.SetTitles("FEATURES", "LOG")

	result := sp.Render("", "")

	if !strings.Contains(result, "FEATURES") {
		t.Error("Render should contain custom left title")
	}
	if !strings.Contains(result, "LOG") {
		t.Error("Render should contain custom right title")
	}
}

func TestSplitPaneHeaderUnderline(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(80, 20)

	result := sp.Render("", "")

	if !strings.Contains(result, "─") {
		t.Error("Render should contain header underline")
	}
}

func TestTruncateLine(t *testing.T) {
	tests := []struct {
		input    string
		maxWidth int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long line", 10, "this is..."},
		{"ab", 2, "ab"},
		{"abc", 3, "..."},
		{"a", 1, "."},
	}

	for _, tt := range tests {
		result := truncateLine(tt.input, tt.maxWidth)
		if len(result) > tt.maxWidth {
			t.Errorf("truncateLine(%q, %d) = %q, length exceeds max", tt.input, tt.maxWidth, result)
		}
	}
}

func TestSplitPaneNarrowWidth(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(25, 10)

	leftWidth := sp.LeftPaneWidth()
	rightWidth := sp.RightPaneWidth()

	if leftWidth < 1 || rightWidth < 1 {
		t.Errorf("Pane widths should be positive: left=%d, right=%d", leftWidth, rightWidth)
	}

	result := sp.Render("test", "test")
	if result == "" {
		t.Error("Should render content even with narrow width")
	}
}
