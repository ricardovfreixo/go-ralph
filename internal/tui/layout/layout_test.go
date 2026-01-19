package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHeaderRender(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:   "test v1.0",
		Title:     "Test PRD",
		AutoMode:  false,
		Total:     5,
		Completed: 2,
		Running:   1,
		Failed:    0,
		Pending:   2,
	}

	result := h.Render(data)

	if !strings.Contains(result, "test v1.0") {
		t.Error("Header should contain version")
	}
	if !strings.Contains(result, "Test PRD") {
		t.Error("Header should contain title")
	}
	if !strings.Contains(result, "2/5 done") {
		t.Error("Header should contain completion summary")
	}
	if !strings.Contains(result, "1 running") {
		t.Error("Header should show running count")
	}
	if !strings.Contains(result, "2 pending") {
		t.Error("Header should show pending count")
	}
}

func TestHeaderAutoMode(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:   "test v1.0",
		Title:     "Test PRD",
		AutoMode:  true,
		Total:     3,
		Completed: 1,
		Running:   0,
		Failed:    0,
		Pending:   2,
	}

	result := h.Render(data)

	if !strings.Contains(result, "[AUTO]") {
		t.Error("Header should show AUTO indicator when autoMode is true")
	}
}

func TestHeaderNoAutoMode(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:   "test v1.0",
		Title:     "Test PRD",
		AutoMode:  false,
		Total:     3,
		Completed: 1,
		Running:   0,
		Failed:    0,
		Pending:   2,
	}

	result := h.Render(data)

	if strings.Contains(result, "[AUTO]") {
		t.Error("Header should not show AUTO indicator when autoMode is false")
	}
}

func TestFooterRender(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)

	data := FooterData{
		Keybindings: "q: quit • ?: help",
		StatusMsg:   "",
	}

	result := f.Render(data)

	if !strings.Contains(result, "q: quit") {
		t.Error("Footer should contain keybindings")
	}
}

func TestFooterWithStatus(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)

	data := FooterData{
		Keybindings: "q: quit",
		StatusMsg:   "Feature started",
		StatusColor: colorRunning,
	}

	result := f.Render(data)

	if !strings.Contains(result, "q: quit") {
		t.Error("Footer should contain keybindings")
	}
	if !strings.Contains(result, "Feature started") {
		t.Error("Footer should contain status message")
	}
}

func TestContainerSetSize(t *testing.T) {
	c := NewContainer()
	c.SetSize(100, 40)

	if c.Width() != 100 {
		t.Errorf("Expected width 100, got %d", c.Width())
	}
	if c.Height() != 40 {
		t.Errorf("Expected height 40, got %d", c.Height())
	}
}

func TestContainerContentHeight(t *testing.T) {
	c := NewContainer()
	c.SetSize(80, 30)

	contentHeight := c.ContentHeight()
	expectedHeight := 30 - HeaderHeight - FooterHeight

	if contentHeight != expectedHeight {
		t.Errorf("Expected content height %d, got %d", expectedHeight, contentHeight)
	}
}

func TestContainerContentHeightMinimum(t *testing.T) {
	c := NewContainer()
	c.SetSize(80, 3)

	contentHeight := c.ContentHeight()

	if contentHeight < 1 {
		t.Error("Content height should be at least 1")
	}
}

func TestLayoutSetSize(t *testing.T) {
	l := New()
	l.SetSize(120, 50)

	if l.Width() != 120 {
		t.Errorf("Expected width 120, got %d", l.Width())
	}
	if l.Height() != 50 {
		t.Errorf("Expected height 50, got %d", l.Height())
	}
}

func TestLayoutContentDimensions(t *testing.T) {
	l := New()
	l.SetSize(100, 40)

	contentHeight := l.ContentHeight()
	contentWidth := l.ContentWidth()

	if contentHeight <= 0 {
		t.Error("Content height should be positive")
	}
	if contentWidth <= 0 {
		t.Error("Content width should be positive")
	}
	if contentHeight >= 40 {
		t.Error("Content height should be less than total height")
	}
}

func TestLayoutRender(t *testing.T) {
	l := New()
	l.SetSize(80, 24)

	headerData := HeaderData{
		Version:   "test v1.0",
		Title:     "Test",
		Total:     3,
		Completed: 1,
		Running:   1,
		Failed:    0,
		Pending:   1,
	}

	footerData := FooterData{
		Keybindings: "q: quit",
	}

	content := "Feature 1\nFeature 2\nFeature 3"

	result := l.Render(headerData, footerData, content)

	if !strings.Contains(result, "test v1.0") {
		t.Error("Layout should render header with version")
	}
	if !strings.Contains(result, "q: quit") {
		t.Error("Layout should render footer with keybindings")
	}
	if !strings.Contains(result, "Feature") {
		t.Error("Layout should render content")
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status   string
		expected lipgloss.TerminalColor
	}{
		{"running", colorRunning},
		{"completed", colorCompleted},
		{"failed", colorFailed},
		{"stopped", colorStopped},
		{"pending", colorPending},
		{"", colorPending},
		{"unknown", colorPending},
	}

	for _, tt := range tests {
		result := StatusColor(tt.status)
		if result != tt.expected {
			t.Errorf("StatusColor(%q): colors don't match", tt.status)
		}
	}
}

func TestHeaderHeight(t *testing.T) {
	h := NewHeader()
	if h.Height() != HeaderHeight {
		t.Errorf("Expected header height %d, got %d", HeaderHeight, h.Height())
	}
}

func TestFooterHeight(t *testing.T) {
	f := NewFooter()
	if f.Height() != FooterHeight {
		t.Errorf("Expected footer height %d, got %d", FooterHeight, f.Height())
	}
}

func TestHeaderSummaryWithFailures(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:   "test v1.0",
		Title:     "Test PRD",
		Total:     5,
		Completed: 2,
		Running:   0,
		Failed:    2,
		Pending:   1,
	}

	result := h.Render(data)

	if !strings.Contains(result, "2 failed") {
		t.Error("Header should show failed count when failures exist")
	}
}

func TestContainerRenderWithLongContent(t *testing.T) {
	c := NewContainer()
	c.SetSize(80, 10)

	headerData := HeaderData{
		Version: "test",
		Title:   "Test",
		Total:   1,
	}

	footerData := FooterData{
		Keybindings: "q: quit",
	}

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "Line"
	}
	content := strings.Join(lines, "\n")

	result := c.Render(headerData, footerData, content)

	if result == "" {
		t.Error("Container should render even with long content")
	}
}
