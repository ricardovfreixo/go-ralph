package layout

import (
	"strings"
	"testing"
)

func TestNewTitleBar(t *testing.T) {
	tb := NewTitleBar()
	if tb == nil {
		t.Fatal("NewTitleBar() returned nil")
	}
	if tb.Title() != "" {
		t.Errorf("expected empty title, got %q", tb.Title())
	}
}

func TestTitleBarSetTitle(t *testing.T) {
	tb := NewTitleBar()
	tb.SetTitle("Test PRD Title")
	if tb.Title() != "Test PRD Title" {
		t.Errorf("expected 'Test PRD Title', got %q", tb.Title())
	}
}

func TestTitleBarHeight(t *testing.T) {
	tb := NewTitleBar()
	if tb.Height() != TitleBarHeight {
		t.Errorf("expected height %d, got %d", TitleBarHeight, tb.Height())
	}
	if tb.Height() != 1 {
		t.Errorf("expected TitleBarHeight to be 1, got %d", tb.Height())
	}
}

func TestTitleBarSetWidth(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(80)
	if tb.width != 80 {
		t.Errorf("expected width 80, got %d", tb.width)
	}
}

func TestTitleBarRenderEmpty(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(80)
	result := tb.Render()
	if result != "" {
		t.Errorf("expected empty string for empty title, got %q", result)
	}
}

func TestTitleBarRenderBasic(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(80)
	tb.SetTitle("My PRD Title")
	result := tb.Render()

	if !strings.Contains(result, "My PRD Title") {
		t.Errorf("expected result to contain 'My PRD Title', got %q", result)
	}
}

func TestTitleBarTruncation(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(20)
	tb.SetTitle("This is a very long PRD title that should be truncated")
	result := tb.Render()

	if !strings.Contains(result, "...") {
		t.Errorf("expected truncated result to contain '...', got %q", result)
	}
	if strings.Contains(result, "should be truncated") {
		t.Errorf("expected long text to be truncated, got %q", result)
	}
}

func TestTitleBarTruncationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		title    string
		wantDots bool
	}{
		{
			name:     "very narrow width",
			width:    5,
			title:    "Long title here",
			wantDots: false,
		},
		{
			name:     "exact fit",
			width:    20,
			title:    "Short",
			wantDots: false,
		},
		{
			name:     "needs truncation",
			width:    15,
			title:    "This needs truncating",
			wantDots: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewTitleBar()
			tb.SetWidth(tt.width)
			tb.SetTitle(tt.title)
			result := tb.Render()

			hasDots := strings.Contains(result, "...")
			if tt.wantDots && !hasDots {
				t.Errorf("expected truncation with '...' but didn't find it in %q", result)
			}
		})
	}
}

func TestTitleBarDistinctFromHeader(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(80)
	tb.SetTitle("Test Title")
	result := tb.Render()

	if result == "" {
		t.Fatal("title bar rendered empty")
	}

	h := NewHeader()
	h.SetWidth(80)
	headerResult := h.Render(HeaderData{
		Version: "v1.0",
		Title:   "Test Title",
	})

	if result == headerResult {
		t.Error("title bar should render differently from header")
	}
}

func TestTitleBarUpdateOnPRDChange(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(80)

	tb.SetTitle("First PRD")
	result1 := tb.Render()
	if !strings.Contains(result1, "First PRD") {
		t.Errorf("expected 'First PRD' in result, got %q", result1)
	}

	tb.SetTitle("Second PRD")
	result2 := tb.Render()
	if !strings.Contains(result2, "Second PRD") {
		t.Errorf("expected 'Second PRD' in result, got %q", result2)
	}
	if strings.Contains(result2, "First PRD") {
		t.Errorf("should not contain old title after update, got %q", result2)
	}
}

func TestTitleBarMinimalWidth(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(3)
	tb.SetTitle("Test")
	result := tb.Render()
	if result == "" {
		t.Error("should render something even at minimal width")
	}
}

func TestTitleBarZeroWidth(t *testing.T) {
	tb := NewTitleBar()
	tb.SetWidth(0)
	tb.SetTitle("Test")
	result := tb.Render()
	if result == "" {
		t.Error("should handle zero width gracefully")
	}
}
