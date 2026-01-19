package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const FooterHeight = 2

type FooterData struct {
	Keybindings string
	StatusMsg   string
	StatusColor lipgloss.TerminalColor
}

type Footer struct {
	width int
}

func NewFooter() *Footer {
	return &Footer{}
}

func (f *Footer) SetWidth(width int) {
	f.width = width
}

func (f *Footer) Height() int {
	return FooterHeight
}

func (f *Footer) Render(data FooterData) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(colorSubtle)

	content := keyStyle.Render(data.Keybindings)

	if data.StatusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(data.StatusColor)
		status := statusStyle.Render(data.StatusMsg)

		contentWidth := f.width - 4
		keysWidth := lipgloss.Width(data.Keybindings)
		statusWidth := lipgloss.Width(data.StatusMsg)

		spaces := contentWidth - keysWidth - statusWidth
		if spaces < 1 {
			content = status + "\n" + content
		} else {
			content = content + strings.Repeat(" ", spaces) + status
		}
	}

	boxStyle := lipgloss.NewStyle().
		Width(f.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorBorder).
		Padding(0, 1)

	return boxStyle.Render(content)
}
