package layout

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewModal(t *testing.T) {
	m := NewModal()
	if m == nil {
		t.Fatal("NewModal() returned nil")
	}
}

func TestModalSetSize(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}

	expectedModalWidth := 100 * ModalWidthPercent / 100
	if m.modalWidth != expectedModalWidth {
		t.Errorf("expected modal width %d, got %d", expectedModalWidth, m.modalWidth)
	}

	expectedModalHeight := 50 * ModalHeightPercent / 100
	if m.modalHeight != expectedModalHeight {
		t.Errorf("expected modal height %d, got %d", expectedModalHeight, m.modalHeight)
	}
}

func TestModalSetSizeMinimums(t *testing.T) {
	m := NewModal()
	m.SetSize(60, 20)

	if m.modalWidth < ModalMinWidth {
		t.Errorf("modal width %d should not be less than minimum %d when terminal is large enough", m.modalWidth, ModalMinWidth)
	}
	if m.modalHeight < ModalMinHeight {
		t.Errorf("modal height %d should not be less than minimum %d when terminal is large enough", m.modalHeight, ModalMinHeight)
	}
}

func TestModalSetSizeSmallTerminal(t *testing.T) {
	m := NewModal()
	m.SetSize(30, 8)

	if m.modalWidth < 1 {
		t.Errorf("modal width should be at least 1, got %d", m.modalWidth)
	}
	if m.modalHeight < 1 {
		t.Errorf("modal height should be at least 1, got %d", m.modalHeight)
	}
}

func TestModalSetTitle(t *testing.T) {
	m := NewModal()
	m.SetTitle("Test Feature")

	if m.title != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got '%s'", m.title)
	}
}

func TestModalSetStatus(t *testing.T) {
	m := NewModal()
	m.SetStatus("running")

	if m.status != "running" {
		t.Errorf("expected status 'running', got '%s'", m.status)
	}
}

func TestModalSetContent(t *testing.T) {
	m := NewModal()
	content := "Line 1\nLine 2\nLine 3"
	m.SetContent(content)

	if m.content != content {
		t.Errorf("expected content '%s', got '%s'", content, m.content)
	}
}

func TestModalSetTestSummary(t *testing.T) {
	m := NewModal()
	summary := "Tests: 5 passed, 0 failed"
	m.SetTestSummary(summary)

	if m.testSummary != summary {
		t.Errorf("expected test summary '%s', got '%s'", summary, m.testSummary)
	}
}

func TestModalContentHeight(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)

	h := m.ContentHeight()
	if h < 1 {
		t.Errorf("content height should be at least 1, got %d", h)
	}

	expectedMax := m.modalHeight - ModalBorderSize - ModalTitleHeight - (ModalPadding * 2)
	if h > expectedMax {
		t.Errorf("content height %d should not exceed %d", h, expectedMax)
	}
}

func TestModalContentHeightWithTestSummary(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)
	heightWithout := m.ContentHeight()

	m.SetTestSummary("Tests: 5 passed")
	heightWith := m.ContentHeight()

	if heightWith >= heightWithout {
		t.Errorf("content height with test summary (%d) should be less than without (%d)", heightWith, heightWithout)
	}
}

func TestModalContentWidth(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)

	w := m.ContentWidth()
	if w < 1 {
		t.Errorf("content width should be at least 1, got %d", w)
	}

	expectedMax := m.modalWidth - ModalBorderSize - (ModalPadding * 2)
	if w > expectedMax {
		t.Errorf("content width %d should not exceed %d", w, expectedMax)
	}
}

func TestModalScrollOffset(t *testing.T) {
	m := NewModal()

	if m.ScrollOffset() != 0 {
		t.Errorf("initial scroll offset should be 0, got %d", m.ScrollOffset())
	}

	m.SetScrollOffset(10)
	if m.ScrollOffset() != 10 {
		t.Errorf("expected scroll offset 10, got %d", m.ScrollOffset())
	}
}

func TestModalScrollOffsetNegative(t *testing.T) {
	m := NewModal()
	m.SetScrollOffset(-5)

	if m.ScrollOffset() != 0 {
		t.Errorf("scroll offset should not be negative, got %d", m.ScrollOffset())
	}
}

func TestModalScrollDown(t *testing.T) {
	m := NewModal()
	m.ScrollDown()

	if m.ScrollOffset() != 1 {
		t.Errorf("expected scroll offset 1 after ScrollDown, got %d", m.ScrollOffset())
	}
}

func TestModalScrollUp(t *testing.T) {
	m := NewModal()
	m.SetScrollOffset(5)
	m.ScrollUp()

	if m.ScrollOffset() != 4 {
		t.Errorf("expected scroll offset 4 after ScrollUp, got %d", m.ScrollOffset())
	}
}

func TestModalScrollUpAtZero(t *testing.T) {
	m := NewModal()
	m.ScrollUp()

	if m.ScrollOffset() != 0 {
		t.Errorf("scroll offset should stay at 0 when scrolling up from 0, got %d", m.ScrollOffset())
	}
}

func TestModalScrollToTop(t *testing.T) {
	m := NewModal()
	m.SetScrollOffset(100)
	m.ScrollToTop()

	if m.ScrollOffset() != 0 {
		t.Errorf("expected scroll offset 0 after ScrollToTop, got %d", m.ScrollOffset())
	}
}

func TestModalScrollToBottom(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)

	totalLines := 100
	m.ScrollToBottom(totalLines)

	expected := totalLines - m.ContentHeight()
	if expected < 0 {
		expected = 0
	}

	if m.ScrollOffset() != expected {
		t.Errorf("expected scroll offset %d after ScrollToBottom, got %d", expected, m.ScrollOffset())
	}
}

func TestModalScrollToBottomFewLines(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)

	m.ScrollToBottom(5)

	if m.ScrollOffset() != 0 {
		t.Errorf("scroll offset should be 0 when total lines < content height, got %d", m.ScrollOffset())
	}
}

func TestModalRenderZeroSize(t *testing.T) {
	m := NewModal()
	background := "Background"

	result := m.Render(background)
	if result != background {
		t.Errorf("expected background unchanged for zero size, got '%s'", result)
	}
}

func TestModalRenderBasic(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.SetTitle("Test Feature")
	m.SetStatus("running")
	m.SetContent("Hello World")

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := m.Render(background)

	if !strings.Contains(result, "Test Feature") {
		t.Error("rendered output should contain the title")
	}
	if !strings.Contains(result, "running") {
		t.Error("rendered output should contain the status")
	}
	if !strings.Contains(result, "Hello World") {
		t.Error("rendered output should contain the content")
	}
}

func TestModalRenderWithTestSummary(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.SetTitle("Test Feature")
	m.SetTestSummary("Tests: 5 passed, 0 failed")
	m.SetContent("Output content")

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := m.Render(background)

	if !strings.Contains(result, "Tests:") {
		t.Error("rendered output should contain the test summary")
	}
}

func TestModalDimBackground(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)

	background := "Some background content"
	dimmed := m.dimBackground(background)

	stripped := stripAnsi(dimmed)
	if stripped != background {
		t.Errorf("stripped dimmed content should equal original, got '%s' vs '%s'", stripped, background)
	}
}

func TestModalDimBackgroundMultiline(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)

	background := "Line 1\nLine 2\nLine 3"
	dimmed := m.dimBackground(background)

	stripped := stripAnsi(dimmed)
	if stripped != background {
		t.Errorf("stripped dimmed content should equal original, got '%s' vs '%s'", stripped, background)
	}

	lines := strings.Split(dimmed, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no escape", "no escape"},
		{"\x1b[38;5;196mcolor\x1b[m", "color"},
	}

	for _, tt := range tests {
		result := stripAnsi(tt.input)
		if result != tt.expected {
			t.Errorf("stripAnsi(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestModalOverlayModal(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)

	background := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23)
	background += strings.Repeat(" ", 80)
	modalBox := "Modal Content"

	result := m.overlayModal(background, modalBox)

	lines := strings.Split(result, "\n")
	if len(lines) < 24 {
		t.Errorf("expected at least 24 lines, got %d", len(lines))
	}
}

func TestModalRenderCentered(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)
	m.SetTitle("Centered")
	m.SetContent("Content")

	var bgLines []string
	for i := 0; i < 50; i++ {
		bgLines = append(bgLines, strings.Repeat(".", 100))
	}
	background := strings.Join(bgLines, "\n")

	result := m.Render(background)
	lines := strings.Split(result, "\n")

	foundBorder := false
	for _, line := range lines {
		if strings.Contains(line, "╭") || strings.Contains(line, "╮") {
			foundBorder = true
			break
		}
	}

	if !foundBorder {
		t.Error("expected to find rounded border in modal")
	}
}

func TestModalScrollingContent(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("Line number %d content here", i))
	}
	m.SetContent(strings.Join(lines, "\n"))

	var bgLines []string
	for i := 0; i < 24; i++ {
		bgLines = append(bgLines, strings.Repeat(".", 80))
	}
	background := strings.Join(bgLines, "\n")

	m.SetScrollOffset(0)
	result1 := m.Render(background)

	m.SetScrollOffset(50)
	result2 := m.Render(background)

	if result1 == result2 {
		t.Error("scrolled content should differ from initial position")
	}

	if !strings.Contains(result1, "Line number 0") {
		t.Error("first render should show first line")
	}
}

func TestModalConstants(t *testing.T) {
	if ModalWidthPercent <= 0 || ModalWidthPercent > 100 {
		t.Errorf("ModalWidthPercent should be between 1 and 100, got %d", ModalWidthPercent)
	}
	if ModalHeightPercent <= 0 || ModalHeightPercent > 100 {
		t.Errorf("ModalHeightPercent should be between 1 and 100, got %d", ModalHeightPercent)
	}
	if ModalMinWidth < 1 {
		t.Errorf("ModalMinWidth should be at least 1, got %d", ModalMinWidth)
	}
	if ModalMinHeight < 1 {
		t.Errorf("ModalMinHeight should be at least 1, got %d", ModalMinHeight)
	}
}

func TestModalSetAutoScroll(t *testing.T) {
	m := NewModal()

	if m.autoScroll {
		t.Error("autoScroll should be false by default")
	}

	m.SetAutoScroll(true)
	if !m.autoScroll {
		t.Error("autoScroll should be true after SetAutoScroll(true)")
	}

	m.SetAutoScroll(false)
	if m.autoScroll {
		t.Error("autoScroll should be false after SetAutoScroll(false)")
	}
}

func TestModalAutoScrollIndicatorFollowing(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.SetTitle("Test Feature")
	m.SetStatus("running")
	m.SetAutoScroll(true)

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := m.Render(background)

	if !strings.Contains(result, "[following]") {
		t.Error("rendered output should contain [following] when autoScroll is enabled")
	}
}

func TestModalAutoScrollIndicatorPaused(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.SetTitle("Test Feature")
	m.SetStatus("running")
	m.SetAutoScroll(false)

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := m.Render(background)

	if !strings.Contains(result, "[paused]") {
		t.Error("rendered output should contain [paused] when autoScroll is disabled")
	}
}

func TestModalSetUsageSummary(t *testing.T) {
	m := NewModal()
	summary := "Input: 1.5k  Output: 800  Total: 2.3k"
	m.SetUsageSummary(summary)

	if m.usageSummary != summary {
		t.Errorf("expected usage summary '%s', got '%s'", summary, m.usageSummary)
	}
}

func TestModalContentHeightWithUsageSummary(t *testing.T) {
	m := NewModal()
	m.SetSize(100, 50)
	heightWithout := m.ContentHeight()

	m.SetUsageSummary("Input: 1.5k  Output: 800")
	heightWith := m.ContentHeight()

	if heightWith >= heightWithout {
		t.Errorf("content height with usage summary (%d) should be less than without (%d)", heightWith, heightWithout)
	}
}

func TestModalRenderWithUsageSummary(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.SetTitle("Test Feature")
	m.SetUsageSummary("Tokens: Input: 1.5k  Output: 800")
	m.SetContent("Output content")

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := m.Render(background)

	if !strings.Contains(result, "Tokens:") {
		t.Error("rendered output should contain the usage summary")
	}
}
