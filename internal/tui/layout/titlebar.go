package layout

import (
	"github.com/charmbracelet/lipgloss"
)

const TitleBarHeight = 1

type TitleBar struct {
	width int
	title string
}

func NewTitleBar() *TitleBar {
	return &TitleBar{}
}

func (t *TitleBar) SetWidth(width int) {
	t.width = width
}

func (t *TitleBar) SetTitle(title string) {
	t.title = title
}

func (t *TitleBar) Title() string {
	return t.title
}

func (t *TitleBar) Height() int {
	return TitleBarHeight
}

func (t *TitleBar) Render() string {
	if t.title == "" {
		return ""
	}

	contentWidth := t.width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	displayTitle := t.title
	if len(displayTitle) > contentWidth {
		if contentWidth > 3 {
			displayTitle = displayTitle[:contentWidth-3] + "..."
		} else {
			displayTitle = displayTitle[:contentWidth]
		}
	}

	style := lipgloss.NewStyle().
		Width(t.width).
		Background(colorTitleBarBg).
		Foreground(colorTitleBarFg).
		Bold(true).
		Padding(0, 1)

	return style.Render(displayTitle)
}
