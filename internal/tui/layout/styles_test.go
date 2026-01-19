package layout

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestAppVersionConstants(t *testing.T) {
	if AppName == "" {
		t.Error("AppName should not be empty")
	}
	if AppVersion == "" {
		t.Error("AppVersion should not be empty")
	}
	if AppName != "RALPH" {
		t.Errorf("Expected AppName to be 'RALPH', got '%s'", AppName)
	}
	if AppVersion != "v0.4.0" {
		t.Errorf("Expected AppVersion to be 'v0.4.0', got '%s'", AppVersion)
	}
}

func TestAdaptiveColors(t *testing.T) {
	colors := []lipgloss.AdaptiveColor{
		colorBorder,
		colorTitle,
		colorSubtle,
		colorNormal,
		colorHighlight,
		colorAuto,
		colorRunning,
		colorCompleted,
		colorFailed,
		colorStopped,
		colorPending,
		colorTitleBarBg,
		colorTitleBarFg,
		colorDim,
		colorPaneFocus,
		colorPaneUnfocus,
		colorSelected,
		colorSelectedFg,
	}

	for i, c := range colors {
		if c.Light == "" {
			t.Errorf("Color %d has empty Light value", i)
		}
		if c.Dark == "" {
			t.Errorf("Color %d has empty Dark value", i)
		}
	}
}

func TestStatusColorReturnsTerminalColor(t *testing.T) {
	statuses := []string{"running", "completed", "failed", "stopped", "pending", "", "unknown"}
	for _, status := range statuses {
		result := StatusColor(status)
		if result == nil {
			t.Errorf("StatusColor(%q) returned nil", status)
		}
	}
}

func TestColorDistinctness(t *testing.T) {
	statusColors := map[string]lipgloss.TerminalColor{
		"running":   colorRunning,
		"completed": colorCompleted,
		"failed":    colorFailed,
		"stopped":   colorStopped,
		"pending":   colorPending,
	}

	for name1, color1 := range statusColors {
		for name2, color2 := range statusColors {
			if name1 != name2 {
				if color1 == color2 {
					t.Errorf("Colors for %q and %q should be distinct", name1, name2)
				}
			}
		}
	}
}

func TestAdaptiveColorLightDarkDistinct(t *testing.T) {
	adaptiveColors := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"running", colorRunning},
		{"completed", colorCompleted},
		{"failed", colorFailed},
		{"stopped", colorStopped},
		{"titleBarBg", colorTitleBarBg},
		{"titleBarFg", colorTitleBarFg},
		{"normal", colorNormal},
		{"dim", colorDim},
	}

	for _, ac := range adaptiveColors {
		if ac.color.Light == "" || ac.color.Dark == "" {
			t.Errorf("Color %q has missing light or dark value", ac.name)
		}
	}
}

func TestLightDarkColorContrast(t *testing.T) {
	if colorTitleBarBg.Light == colorTitleBarFg.Light {
		t.Error("Light mode title bar bg and fg should be different")
	}
	if colorTitleBarBg.Dark == colorTitleBarFg.Dark {
		t.Error("Dark mode title bar bg and fg should be different")
	}

	if colorNormal.Light == colorDim.Light {
		t.Error("Light mode normal and dim colors should be different")
	}
	if colorNormal.Dark == colorDim.Dark {
		t.Error("Dark mode normal and dim colors should be different")
	}
}

func TestColorPaneFocusDistinct(t *testing.T) {
	if colorPaneFocus == colorPaneUnfocus {
		t.Error("Pane focus colors should be distinct")
	}
	if colorPaneFocus.Light == colorPaneUnfocus.Light {
		t.Error("Light mode pane focus colors should be different")
	}
	if colorPaneFocus.Dark == colorPaneUnfocus.Dark {
		t.Error("Dark mode pane focus colors should be different")
	}
}
