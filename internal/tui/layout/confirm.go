package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	ConfirmMinWidth = 30
	ConfirmMaxWidth = 60
	ConfirmPadding  = 2
	ConfirmBorderSz = 2
)

type ConfirmType int

const (
	ConfirmTypeQuit ConfirmType = iota
	ConfirmTypeReset
	ConfirmTypeBudget
)

type ConfirmDialog struct {
	width      int
	height     int
	dialogType ConfirmType
	visible    bool
}

func NewConfirmDialog() *ConfirmDialog {
	return &ConfirmDialog{}
}

func (c *ConfirmDialog) SetSize(width, height int) {
	c.width = width
	c.height = height
}

func (c *ConfirmDialog) Show(dialogType ConfirmType) {
	c.dialogType = dialogType
	c.visible = true
}

func (c *ConfirmDialog) Hide() {
	c.visible = false
}

func (c *ConfirmDialog) IsVisible() bool {
	return c.visible
}

func (c *ConfirmDialog) Type() ConfirmType {
	return c.dialogType
}

func (c *ConfirmDialog) title() string {
	switch c.dialogType {
	case ConfirmTypeQuit:
		return "Quit ralph?"
	case ConfirmTypeReset:
		return "Reset ALL features?"
	case ConfirmTypeBudget:
		return "Budget threshold reached!"
	default:
		return "Confirm"
	}
}

func (c *ConfirmDialog) message() string {
	switch c.dialogType {
	case ConfirmTypeQuit:
		return "Progress will be saved."
	case ConfirmTypeReset:
		return "This will stop all instances and\ndelete progress.md"
	case ConfirmTypeBudget:
		return "You've used 90% of your budget.\nContinue anyway?"
	default:
		return ""
	}
}

func (c *ConfirmDialog) dialogWidth() int {
	title := c.title()
	msg := c.message()

	maxLen := len(title)
	for _, line := range strings.Split(msg, "\n") {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	w := maxLen + (ConfirmPadding * 2) + ConfirmBorderSz + 4
	if w < ConfirmMinWidth {
		w = ConfirmMinWidth
	}
	if w > ConfirmMaxWidth {
		w = ConfirmMaxWidth
	}
	if w > c.width-4 {
		w = c.width - 4
	}
	return w
}

func (c *ConfirmDialog) Render(background string) string {
	if !c.visible || c.width <= 0 || c.height <= 0 {
		return background
	}

	dimmedBg := c.dimBackground(background)
	dialogBox := c.renderDialogBox()
	return c.overlayDialog(dimmedBg, dialogBox)
}

func (c *ConfirmDialog) dimBackground(background string) string {
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	lines := strings.Split(background, "\n")
	var dimmed []string
	for _, line := range lines {
		stripped := stripAnsi(line)
		dimmed = append(dimmed, dimStyle.Render(stripped))
	}
	return strings.Join(dimmed, "\n")
}

func (c *ConfirmDialog) renderDialogBox() string {
	dw := c.dialogWidth()
	innerWidth := dw - ConfirmBorderSz - (ConfirmPadding * 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorFailed).
		Width(innerWidth).
		Align(lipgloss.Center)

	msgStyle := lipgloss.NewStyle().
		Foreground(colorNormal).
		Width(innerWidth).
		Align(lipgloss.Center)

	keysStyle := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Width(innerWidth).
		Align(lipgloss.Center)

	title := titleStyle.Render(c.title())
	msg := msgStyle.Render(c.message())
	keys := keysStyle.Render("y: yes • n/Esc: no")

	content := title + "\n\n" + msg + "\n\n" + keys

	borderColor := colorFailed
	if c.dialogType == ConfirmTypeQuit {
		borderColor = colorRunning
	} else if c.dialogType == ConfirmTypeBudget {
		borderColor = lipgloss.AdaptiveColor{Light: "208", Dark: "208"}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, ConfirmPadding)

	return boxStyle.Render(content)
}

func (c *ConfirmDialog) overlayDialog(background, dialog string) string {
	bgLines := strings.Split(background, "\n")
	dialogLines := strings.Split(dialog, "\n")

	dw := lipgloss.Width(dialogLines[0])
	dh := len(dialogLines)

	for len(bgLines) < c.height {
		bgLines = append(bgLines, strings.Repeat(" ", c.width))
	}

	startY := (c.height - dh) / 2
	startX := (c.width - dw) / 2
	if startX < 0 {
		startX = 0
	}

	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	for i, dialogLine := range dialogLines {
		bgIdx := startY + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgIdx]
		bgRunes := []rune(stripAnsi(bgLine))

		for len(bgRunes) < c.width {
			bgRunes = append(bgRunes, ' ')
		}

		var newLine string
		if startX > 0 {
			newLine = dimStyle.Render(string(bgRunes[:startX]))
		}
		newLine += dialogLine

		dialogWidth := lipgloss.Width(dialogLine)
		endX := startX + dialogWidth
		if endX < c.width && endX < len(bgRunes) {
			newLine += dimStyle.Render(string(bgRunes[endX:]))
		}

		bgLines[bgIdx] = newLine
	}

	return strings.Join(bgLines, "\n")
}
