package layout

import (
	"strings"
	"testing"
)

func TestNewHelpModal(t *testing.T) {
	h := NewHelpModal()
	if h == nil {
		t.Fatal("NewHelpModal returned nil")
	}
	if h.visible {
		t.Error("HelpModal should not be visible by default")
	}
}

func TestHelpModal_SetSize(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 40)

	if h.width != 100 {
		t.Errorf("expected width 100, got %d", h.width)
	}
	if h.height != 40 {
		t.Errorf("expected height 40, got %d", h.height)
	}
	if h.modalWidth <= 0 {
		t.Error("modalWidth should be positive")
	}
	if h.modalHeight <= 0 {
		t.Error("modalHeight should be positive")
	}
}

func TestHelpModal_ModalSizeConstraints(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		expectWidth   int
		expectHeight  int
	}{
		{
			name:         "normal terminal",
			width:        100,
			height:       40,
			expectWidth:  HelpModalMaxWidth,
			expectHeight: HelpModalMaxHeight,
		},
		{
			name:         "small terminal width",
			width:        50,
			height:       40,
			expectWidth:  48,
			expectHeight: HelpModalMaxHeight,
		},
		{
			name:         "small terminal height",
			width:        100,
			height:       15,
			expectWidth:  HelpModalMaxWidth,
			expectHeight: 13,
		},
		{
			name:         "very small terminal",
			width:        30,
			height:       12,
			expectWidth:  28,
			expectHeight: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHelpModal()
			h.SetSize(tt.width, tt.height)

			if h.modalWidth > tt.expectWidth+1 {
				t.Errorf("expected modalWidth <= %d, got %d", tt.expectWidth, h.modalWidth)
			}
			if h.modalHeight > tt.expectHeight+1 {
				t.Errorf("expected modalHeight <= %d, got %d", tt.expectHeight, h.modalHeight)
			}
		})
	}
}

func TestHelpModal_ShowHide(t *testing.T) {
	h := NewHelpModal()

	if h.IsVisible() {
		t.Error("should not be visible initially")
	}

	h.Show()
	if !h.IsVisible() {
		t.Error("should be visible after Show()")
	}

	h.Hide()
	if h.IsVisible() {
		t.Error("should not be visible after Hide()")
	}
}

func TestHelpModal_Toggle(t *testing.T) {
	h := NewHelpModal()

	h.Toggle()
	if !h.IsVisible() {
		t.Error("should be visible after first Toggle()")
	}

	h.Toggle()
	if h.IsVisible() {
		t.Error("should not be visible after second Toggle()")
	}
}

func TestHelpModal_ScrollingWhenNotNeeded(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 85) // Larger terminal to fit expanded help content with model escalation section
	h.Show()

	if h.NeedsScrolling() {
		t.Error("should not need scrolling with large terminal")
	}

	h.ScrollDown()
	if h.scrollOffset > 0 {
		h.scrollOffset = 0
	}
}

func TestHelpModal_ScrollingWhenNeeded(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 15)
	h.Show()

	if !h.NeedsScrolling() {
		t.Error("should need scrolling with small terminal")
	}

	h.ScrollDown()
	if h.scrollOffset != 1 {
		t.Errorf("expected scrollOffset 1, got %d", h.scrollOffset)
	}

	h.ScrollUp()
	if h.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0, got %d", h.scrollOffset)
	}

	h.ScrollUp()
	if h.scrollOffset != 0 {
		t.Error("scrollOffset should not go negative")
	}
}

func TestHelpModal_ScrollToTop(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 15)
	h.Show()

	h.scrollOffset = 5
	h.ScrollToTop()
	if h.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0, got %d", h.scrollOffset)
	}
}

func TestHelpModal_ScrollToBottom(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 15)
	h.Show()

	h.ScrollToBottom()

	contentLines := strings.Split(helpContent, "\n")
	maxOffset := len(contentLines) - h.ContentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}

	if h.scrollOffset != maxOffset {
		t.Errorf("expected scrollOffset %d, got %d", maxOffset, h.scrollOffset)
	}
}

func TestHelpModal_ContentDimensions(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 40)

	contentWidth := h.ContentWidth()
	contentHeight := h.ContentHeight()

	if contentWidth <= 0 {
		t.Error("ContentWidth should be positive")
	}
	if contentHeight <= 0 {
		t.Error("ContentHeight should be positive")
	}

	if contentWidth >= h.modalWidth {
		t.Error("ContentWidth should be less than modalWidth")
	}
	if contentHeight >= h.modalHeight {
		t.Error("ContentHeight should be less than modalHeight")
	}
}

func TestHelpModal_RenderWhenNotVisible(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(80, 24)

	background := "background content"
	result := h.Render(background)

	if result != background {
		t.Error("should return background unchanged when not visible")
	}
}

func TestHelpModal_RenderWhenVisible(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 40)
	h.Show()

	background := strings.Repeat(strings.Repeat("x", 100)+"\n", 40)
	result := h.Render(background)

	if result == background {
		t.Error("should render modal overlay when visible")
	}
	if !strings.Contains(result, "Help") {
		t.Error("rendered output should contain title 'Help'")
	}
}

func TestHelpModal_RenderContainsKeybindings(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 80) // Increased height to fit expanded help content
	h.Show()

	background := strings.Repeat(strings.Repeat(" ", 100)+"\n", 80)
	result := h.Render(background)

	expectedKeybindings := []string{
		"j/k",
		"Enter",
		"Start",
	}

	for _, kb := range expectedKeybindings {
		if !strings.Contains(result, kb) {
			t.Errorf("rendered output should contain keybinding '%s'", kb)
		}
	}
}

func TestHelpModal_RenderWithZeroSize(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(0, 0)
	h.Show()

	background := "background"
	result := h.Render(background)

	if result != background {
		t.Error("should return background unchanged with zero dimensions")
	}
}

func TestHelpModal_ShowResetsScrollOffset(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 15)

	h.Show()
	h.scrollOffset = 5
	h.Hide()

	h.Show()
	if h.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after Show(), got %d", h.scrollOffset)
	}
}

func TestHelpModal_DimBackground(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(80, 24)

	background := "line1\nline2\nline3"
	dimmed := h.dimBackground(background)

	lines := strings.Split(dimmed, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestHelpModal_OverlayCentering(t *testing.T) {
	h := NewHelpModal()
	h.SetSize(100, 40)
	h.Show()

	startY := (h.height - h.modalHeight) / 2
	startX := (h.width - h.modalWidth) / 2

	if startY < 0 || startY > h.height/2 {
		t.Errorf("modal Y position %d seems incorrect", startY)
	}
	if startX < 0 || startX > h.width/2 {
		t.Errorf("modal X position %d seems incorrect", startX)
	}
}

func TestHelpModal_HelpContentNotEmpty(t *testing.T) {
	if len(helpContent) == 0 {
		t.Error("helpContent should not be empty")
	}

	lines := strings.Split(helpContent, "\n")
	if len(lines) < 10 {
		t.Errorf("helpContent should have at least 10 lines, got %d", len(lines))
	}
}

func TestHelpModal_NeedsScrollingLogic(t *testing.T) {
	tests := []struct {
		name           string
		height         int
		expectedScroll bool
	}{
		{
			name:           "large terminal - no scrolling",
			height:         80, // Larger terminal to fit expanded help content
			expectedScroll: false,
		},
		{
			name:           "small terminal - needs scrolling",
			height:         15,
			expectedScroll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHelpModal()
			h.SetSize(100, tt.height)

			if h.NeedsScrolling() != tt.expectedScroll {
				t.Errorf("expected NeedsScrolling() = %v, got %v", tt.expectedScroll, h.NeedsScrolling())
			}
		})
	}
}
