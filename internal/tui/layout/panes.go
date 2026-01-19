package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	PaneHeaderHeight = 1
	DividerWidth     = 1
	MinPaneWidth     = 10
)

type PaneFocus int

const (
	FocusLeft PaneFocus = iota
	FocusRight
)

type SplitPane struct {
	width      int
	height     int
	leftTitle  string
	rightTitle string
	focus      PaneFocus
}

func NewSplitPane() *SplitPane {
	return &SplitPane{
		leftTitle:  "TASKS",
		rightTitle: "ACTIVITY",
	}
}

func (s *SplitPane) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *SplitPane) SetTitles(left, right string) {
	s.leftTitle = left
	s.rightTitle = right
}

func (s *SplitPane) SetFocus(focus PaneFocus) {
	s.focus = focus
}

func (s *SplitPane) Focus() PaneFocus {
	return s.focus
}

func (s *SplitPane) Width() int {
	return s.width
}

func (s *SplitPane) Height() int {
	return s.height
}

func (s *SplitPane) LeftPaneWidth() int {
	availableWidth := s.width - DividerWidth
	if availableWidth < MinPaneWidth*2 {
		return s.width / 2
	}
	return availableWidth / 2
}

func (s *SplitPane) RightPaneWidth() int {
	availableWidth := s.width - DividerWidth
	leftWidth := s.LeftPaneWidth()
	remaining := availableWidth - leftWidth
	if remaining < MinPaneWidth {
		return MinPaneWidth
	}
	return remaining
}

func (s *SplitPane) ContentHeight() int {
	h := s.height - PaneHeaderHeight
	if h < 1 {
		return 1
	}
	return h
}

func (s *SplitPane) Render(leftContent, rightContent string) string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	leftWidth := s.LeftPaneWidth()
	rightWidth := s.RightPaneWidth()
	contentHeight := s.ContentHeight()

	leftHeader := s.renderPaneHeader(s.leftTitle, leftWidth, s.focus == FocusLeft)
	rightHeader := s.renderPaneHeader(s.rightTitle, rightWidth, s.focus == FocusRight)

	leftBody := s.renderPaneContent(leftContent, leftWidth, contentHeight)
	rightBody := s.renderPaneContent(rightContent, rightWidth, contentHeight)

	var lines []string
	leftHeaderLines := strings.Split(leftHeader, "\n")
	rightHeaderLines := strings.Split(rightHeader, "\n")
	leftBodyLines := strings.Split(leftBody, "\n")
	rightBodyLines := strings.Split(rightBody, "\n")

	dividerStyle := lipgloss.NewStyle().Foreground(colorBorder)
	divider := dividerStyle.Render("│")

	for i := 0; i < len(leftHeaderLines) && i < len(rightHeaderLines); i++ {
		lines = append(lines, leftHeaderLines[i]+divider+rightHeaderLines[i])
	}

	for i := 0; i < contentHeight; i++ {
		leftLine := ""
		rightLine := ""
		if i < len(leftBodyLines) {
			leftLine = leftBodyLines[i]
		}
		if i < len(rightBodyLines) {
			rightLine = rightBodyLines[i]
		}
		lines = append(lines, leftLine+divider+rightLine)
	}

	return strings.Join(lines, "\n")
}

func (s *SplitPane) renderPaneHeader(title string, width int, focused bool) string {
	var titleColor lipgloss.TerminalColor
	var underlineColor lipgloss.TerminalColor
	if focused {
		titleColor = colorPaneFocus
		underlineColor = colorPaneFocus
	} else {
		titleColor = colorPaneUnfocus
		underlineColor = colorBorder
	}

	titleStyle := lipgloss.NewStyle().
		Bold(focused).
		Foreground(titleColor)

	underlineStyle := lipgloss.NewStyle().
		Foreground(underlineColor)

	styledTitle := titleStyle.Render(title)
	titleLen := lipgloss.Width(styledTitle)

	padding := width - titleLen
	if padding < 0 {
		padding = 0
	}

	headerLine := styledTitle + strings.Repeat(" ", padding)

	underline := underlineStyle.Render(strings.Repeat("─", width))

	return headerLine + "\n" + underline
}

func (s *SplitPane) renderPaneContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	if len(lines) > height {
		lines = lines[:height]
	}

	var result []string
	for _, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > width {
			line = truncateLine(line, width)
			lineWidth = lipgloss.Width(line)
		}
		padding := width - lineWidth
		if padding < 0 {
			padding = 0
		}
		result = append(result, line+strings.Repeat(" ", padding))
	}

	for len(result) < height {
		result = append(result, strings.Repeat(" ", width))
	}

	return strings.Join(result, "\n")
}

func truncateLine(line string, maxWidth int) string {
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}

	runes := []rune(line)
	if len(runes) <= maxWidth {
		return line
	}

	return string(runes[:maxWidth-3]) + "..."
}
