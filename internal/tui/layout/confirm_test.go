package layout

import (
	"strings"
	"testing"
)

func TestNewConfirmDialog(t *testing.T) {
	c := NewConfirmDialog()
	if c == nil {
		t.Fatal("NewConfirmDialog() returned nil")
	}
	if c.IsVisible() {
		t.Error("new dialog should not be visible")
	}
}

func TestConfirmDialogSetSize(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(100, 50)

	if c.width != 100 {
		t.Errorf("expected width 100, got %d", c.width)
	}
	if c.height != 50 {
		t.Errorf("expected height 50, got %d", c.height)
	}
}

func TestConfirmDialogShowHide(t *testing.T) {
	c := NewConfirmDialog()

	c.Show(ConfirmTypeQuit)
	if !c.IsVisible() {
		t.Error("dialog should be visible after Show()")
	}
	if c.Type() != ConfirmTypeQuit {
		t.Errorf("expected type ConfirmTypeQuit, got %v", c.Type())
	}

	c.Hide()
	if c.IsVisible() {
		t.Error("dialog should not be visible after Hide()")
	}
}

func TestConfirmDialogTypes(t *testing.T) {
	c := NewConfirmDialog()

	c.Show(ConfirmTypeQuit)
	if c.Type() != ConfirmTypeQuit {
		t.Errorf("expected ConfirmTypeQuit, got %v", c.Type())
	}

	c.Show(ConfirmTypeReset)
	if c.Type() != ConfirmTypeReset {
		t.Errorf("expected ConfirmTypeReset, got %v", c.Type())
	}
}

func TestConfirmDialogTitles(t *testing.T) {
	c := NewConfirmDialog()

	c.Show(ConfirmTypeQuit)
	title := c.title()
	if title != "Quit ralph?" {
		t.Errorf("expected 'Quit ralph?', got '%s'", title)
	}

	c.Show(ConfirmTypeReset)
	title = c.title()
	if title != "Reset ALL features?" {
		t.Errorf("expected 'Reset ALL features?', got '%s'", title)
	}
}

func TestConfirmDialogMessages(t *testing.T) {
	c := NewConfirmDialog()

	c.Show(ConfirmTypeQuit)
	msg := c.message()
	if !strings.Contains(msg, "Progress will be saved") {
		t.Errorf("quit message should mention progress saving, got '%s'", msg)
	}

	c.Show(ConfirmTypeReset)
	msg = c.message()
	if !strings.Contains(msg, "progress.md") {
		t.Errorf("reset message should mention progress.md, got '%s'", msg)
	}
}

func TestConfirmDialogWidth(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(100, 50)
	c.Show(ConfirmTypeQuit)

	w := c.dialogWidth()
	if w < ConfirmMinWidth {
		t.Errorf("dialog width %d should be at least minimum %d", w, ConfirmMinWidth)
	}
	if w > ConfirmMaxWidth {
		t.Errorf("dialog width %d should not exceed maximum %d", w, ConfirmMaxWidth)
	}
}

func TestConfirmDialogWidthSmallTerminal(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(40, 20)
	c.Show(ConfirmTypeQuit)

	w := c.dialogWidth()
	if w > 36 {
		t.Errorf("dialog width %d should be constrained by terminal width", w)
	}
}

func TestConfirmDialogRenderNotVisible(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(80, 24)

	background := "Background content"
	result := c.Render(background)

	if result != background {
		t.Errorf("render with invisible dialog should return unchanged background")
	}
}

func TestConfirmDialogRenderZeroSize(t *testing.T) {
	c := NewConfirmDialog()
	c.Show(ConfirmTypeQuit)

	background := "Background"
	result := c.Render(background)

	if result != background {
		t.Errorf("render with zero size should return unchanged background")
	}
}

func TestConfirmDialogRenderQuit(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(80, 24)
	c.Show(ConfirmTypeQuit)

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := c.Render(background)

	if !strings.Contains(result, "Quit ralph?") {
		t.Error("rendered output should contain quit title")
	}
	if !strings.Contains(result, "y: yes") {
		t.Error("rendered output should contain y/n instructions")
	}
}

func TestConfirmDialogRenderReset(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(80, 24)
	c.Show(ConfirmTypeReset)

	background := strings.Repeat(strings.Repeat("X", 80)+"\n", 24)
	result := c.Render(background)

	if !strings.Contains(result, "Reset ALL features?") {
		t.Error("rendered output should contain reset title")
	}
	if !strings.Contains(result, "progress.md") {
		t.Error("rendered output should mention progress.md")
	}
}

func TestConfirmDialogDimBackground(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(80, 24)

	background := "Some background"
	dimmed := c.dimBackground(background)

	stripped := stripAnsi(dimmed)
	if stripped != background {
		t.Errorf("stripped dimmed content should equal original, got '%s' vs '%s'", stripped, background)
	}
}

func TestConfirmDialogCentered(t *testing.T) {
	c := NewConfirmDialog()
	c.SetSize(100, 50)
	c.Show(ConfirmTypeQuit)

	var bgLines []string
	for i := 0; i < 50; i++ {
		bgLines = append(bgLines, strings.Repeat(".", 100))
	}
	background := strings.Join(bgLines, "\n")

	result := c.Render(background)
	lines := strings.Split(result, "\n")

	foundBorder := false
	for _, line := range lines {
		if strings.Contains(line, "╭") || strings.Contains(line, "╮") {
			foundBorder = true
			break
		}
	}

	if !foundBorder {
		t.Error("expected to find rounded border in dialog")
	}
}

func TestConfirmDialogConstants(t *testing.T) {
	if ConfirmMinWidth < 1 {
		t.Errorf("ConfirmMinWidth should be at least 1, got %d", ConfirmMinWidth)
	}
	if ConfirmMaxWidth < ConfirmMinWidth {
		t.Errorf("ConfirmMaxWidth (%d) should be >= ConfirmMinWidth (%d)", ConfirmMaxWidth, ConfirmMinWidth)
	}
}
