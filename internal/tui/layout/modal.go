package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	ModalWidthPercent  = 80
	ModalHeightPercent = 70
	ModalMinWidth      = 40
	ModalMinHeight     = 10
	ModalBorderSize    = 2
	ModalTitleHeight   = 1
	ModalPadding       = 1
)

type Modal struct {
	width             int
	height            int
	modalWidth        int
	modalHeight       int
	title             string
	status            string
	scrollOffset      int
	content           string
	testSummary       string
	usageSummary      string
	adjustmentSummary string
	autoScroll        bool
	showActions       bool
	actionTimeline    string
}

func NewModal() *Modal {
	return &Modal{}
}

func (m *Modal) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.calculateModalSize()
}

func (m *Modal) calculateModalSize() {
	m.modalWidth = m.width * ModalWidthPercent / 100
	if m.modalWidth > m.width-2 && m.width > 2 {
		m.modalWidth = m.width - 2
	}
	if m.modalWidth < ModalMinWidth && m.width >= ModalMinWidth {
		m.modalWidth = ModalMinWidth
	}
	if m.modalWidth < 1 {
		m.modalWidth = 1
	}

	m.modalHeight = m.height * ModalHeightPercent / 100
	if m.modalHeight > m.height-2 && m.height > 2 {
		m.modalHeight = m.height - 2
	}
	if m.modalHeight < ModalMinHeight && m.height >= ModalMinHeight {
		m.modalHeight = ModalMinHeight
	}
	if m.modalHeight < 1 {
		m.modalHeight = 1
	}
}

func (m *Modal) SetTitle(title string) {
	m.title = title
}

func (m *Modal) SetStatus(status string) {
	m.status = status
}

func (m *Modal) SetContent(content string) {
	m.content = content
}

func (m *Modal) SetTestSummary(summary string) {
	m.testSummary = summary
}

func (m *Modal) SetUsageSummary(summary string) {
	m.usageSummary = summary
}

func (m *Modal) SetAdjustmentSummary(summary string) {
	m.adjustmentSummary = summary
}

func (m *Modal) ContentHeight() int {
	h := m.modalHeight - ModalBorderSize - ModalTitleHeight - (ModalPadding * 2)
	if m.testSummary != "" {
		h -= 2
	}
	if m.usageSummary != "" {
		h -= 2
	}
	if m.adjustmentSummary != "" {
		h -= 2
	}
	if h < 1 {
		return 1
	}
	return h
}

func (m *Modal) ContentWidth() int {
	w := m.modalWidth - ModalBorderSize - (ModalPadding * 2)
	if w < 1 {
		return 1
	}
	return w
}

func (m *Modal) ScrollOffset() int {
	return m.scrollOffset
}

func (m *Modal) SetScrollOffset(offset int) {
	m.scrollOffset = offset
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *Modal) SetAutoScroll(enabled bool) {
	m.autoScroll = enabled
}

func (m *Modal) ScrollDown() {
	m.scrollOffset++
}

func (m *Modal) ScrollUp() {
	if m.scrollOffset > 0 {
		m.scrollOffset--
	}
}

func (m *Modal) ScrollToTop() {
	m.scrollOffset = 0
}

func (m *Modal) ScrollToBottom(totalLines int) {
	maxOffset := totalLines - m.ContentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.scrollOffset = maxOffset
}

func (m *Modal) SetActionTimeline(timeline string) {
	m.actionTimeline = timeline
}

func (m *Modal) ToggleActions() bool {
	m.showActions = !m.showActions
	m.scrollOffset = 0
	return m.showActions
}

func (m *Modal) ShowingActions() bool {
	return m.showActions
}

func (m *Modal) ResetView() {
	m.showActions = false
	m.scrollOffset = 0
}

func (m *Modal) Render(background string) string {
	if m.width <= 0 || m.height <= 0 {
		return background
	}

	dimmedBg := m.dimBackground(background)
	modalBox := m.renderModalBox()
	return m.overlayModal(dimmedBg, modalBox)
}

func (m *Modal) dimBackground(background string) string {
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	lines := strings.Split(background, "\n")
	var dimmed []string
	for _, line := range lines {
		stripped := stripAnsi(line)
		dimmed = append(dimmed, dimStyle.Render(stripped))
	}
	return strings.Join(dimmed, "\n")
}

func (m *Modal) renderModalBox() string {
	titleBarStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorTitleBarFg).
		Background(colorTitleBarBg).
		Width(m.modalWidth-ModalBorderSize).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(StatusColor(m.status))

	scrollIndicatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	viewModeStyle := lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true)

	titleText := m.title
	if m.status != "" {
		titleText += " " + statusStyle.Render("["+m.status+"]")
	}
	if m.showActions {
		titleText += " " + viewModeStyle.Render("[ACTIONS]")
	} else if m.autoScroll {
		titleText += " " + scrollIndicatorStyle.Render("[following]")
	} else {
		titleText += " " + scrollIndicatorStyle.Render("[paused]")
	}
	titleBar := titleBarStyle.Render(titleText)

	contentLines := m.renderContent()

	innerContent := titleBar + "\n" + contentLines

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorHighlight).
		Width(m.modalWidth-ModalBorderSize).
		Height(m.modalHeight-ModalBorderSize).
		Padding(0, ModalPadding)

	return boxStyle.Render(innerContent)
}

func (m *Modal) renderContent() string {
	contentWidth := m.ContentWidth()
	contentHeight := m.ContentHeight()

	var lines []string

	if m.showActions {
		if m.actionTimeline == "" {
			lines = append(lines, "No actions recorded yet.")
		} else {
			actionLines := strings.Split(m.actionTimeline, "\n")
			lines = append(lines, actionLines...)
		}
	} else {
		if m.testSummary != "" {
			lines = append(lines, m.testSummary)
			lines = append(lines, "")
		}
		if m.usageSummary != "" {
			lines = append(lines, m.usageSummary)
			lines = append(lines, "")
		}
		if m.adjustmentSummary != "" {
			adjStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			lines = append(lines, adjStyle.Render("Adjustments: "+m.adjustmentSummary))
			lines = append(lines, "")
		}
		contentLines := strings.Split(m.content, "\n")
		lines = append(lines, contentLines...)
	}

	totalLines := len(lines)
	scrollOffset := m.scrollOffset
	if scrollOffset > totalLines-contentHeight {
		scrollOffset = totalLines - contentHeight
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	}

	endIdx := scrollOffset + contentHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	var visibleLines []string
	if scrollOffset < totalLines {
		visibleLines = lines[scrollOffset:endIdx]
	}

	var rendered []string
	for _, line := range visibleLines {
		if lipgloss.Width(line) > contentWidth {
			line = truncateLine(line, contentWidth)
		}
		rendered = append(rendered, line)
	}

	for len(rendered) < contentHeight {
		rendered = append(rendered, "")
	}

	return strings.Join(rendered, "\n")
}

func (m *Modal) overlayModal(background, modal string) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	for len(bgLines) < m.height {
		bgLines = append(bgLines, strings.Repeat(" ", m.width))
	}

	startY := (m.height - m.modalHeight) / 2
	startX := (m.width - m.modalWidth) / 2

	for i, modalLine := range modalLines {
		bgIdx := startY + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgIdx]
		bgRunes := []rune(stripAnsi(bgLine))

		for len(bgRunes) < m.width {
			bgRunes = append(bgRunes, ' ')
		}

		dimStyle := lipgloss.NewStyle().Foreground(colorDim)

		var newLine string
		if startX > 0 {
			newLine = dimStyle.Render(string(bgRunes[:startX]))
		}
		newLine += modalLine

		modalWidth := lipgloss.Width(modalLine)
		endX := startX + modalWidth
		if endX < m.width {
			newLine += dimStyle.Render(string(bgRunes[endX:]))
		}

		bgLines[bgIdx] = newLine
	}

	return strings.Join(bgLines, "\n")
}

func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
