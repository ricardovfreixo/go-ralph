package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	HelpModalMinWidth  = 60
	HelpModalMinHeight = 20
	HelpModalMaxWidth  = 80
	HelpModalMaxHeight = 80 // Increased to fit expanded help content with model escalation section
)

var helpContent = `Navigation:
  j/k or ↑/↓    Move selection up/down
  Enter         Inspect selected feature's output
  Space         Toggle expand/collapse (features with children)

Actions:
  s             Start selected feature
  S             Start ALL features (auto mode)
  r             Retry failed/completed feature
  R             Reset feature (clear attempts)
  x             Stop selected feature
  X             Stop ALL features (exit auto mode)
  Ctrl+r        Reset ALL features (start fresh)

Display:
  c             Toggle cost display (shows $ instead of tokens)
  a             Toggle action timeline (in inspect view)

Tree View:
  Features with sub-features show as expandable trees:
  ▼ ○  Parent Feature (expanded)
  │   └── ○  Child Feature
  ▶ ○  Collapsed Feature (2/3 done)

  Use Space to toggle expand/collapse.
  Collapsed parents show aggregated child status.

Inspect View:
  j/k or ↑/↓    Scroll output
  g             Go to top (pauses auto-scroll)
  G             Go to end (enables auto-scroll)
  f             Follow output (enables auto-scroll)
  a             Toggle action timeline
  s             Start feature
  x             Stop feature
  q/Esc         Close inspect view

General:
  ?             Toggle help
  q             Quit (saves progress)

Auto Mode:
  When started with 'S', ralph will:
  - Start features in order
  - Run up to 3 features in parallel
  - Auto-retry failed features (up to 3 attempts)
  - Stop when all complete or max retries exceeded

Cost Estimation:
  Costs are calculated per model pricing:
  - Sonnet: $3/M input, $15/M output
  - Haiku: $0.80/M input, $4/M output
  - Opus: $15/M input, $75/M output
  Cache tokens have separate pricing.

Model Escalation:
  When Model: auto is specified, ralph automatically
  adjusts the model based on task complexity:

  Escalate to stronger model when:
  - 2+ errors occur at current level
  - Claude explicitly requests (e.g., "needs opus")
  - Architectural keywords detected

  De-escalate to simpler model when:
  - Task is simple (tests, formatting, linting)
  - No errors have occurred
  - Explicit request detected

  Model indicators in feature list:
  - [H] = Haiku, [S] = Sonnet, [O] = Opus
  - Orange = model was changed during execution`

type HelpModal struct {
	width        int
	height       int
	modalWidth   int
	modalHeight  int
	scrollOffset int
	visible      bool
}

func NewHelpModal() *HelpModal {
	return &HelpModal{}
}

func (h *HelpModal) SetSize(width, height int) {
	h.width = width
	h.height = height
	h.calculateModalSize()
}

func (h *HelpModal) calculateModalSize() {
	contentLines := strings.Split(helpContent, "\n")
	contentHeight := len(contentLines)

	h.modalHeight = contentHeight + 4
	if h.modalHeight > HelpModalMaxHeight {
		h.modalHeight = HelpModalMaxHeight
	}
	if h.modalHeight < HelpModalMinHeight {
		h.modalHeight = HelpModalMinHeight
	}
	if h.modalHeight > h.height-2 {
		h.modalHeight = h.height - 2
	}
	if h.modalHeight < 1 {
		h.modalHeight = 1
	}

	var maxLineWidth int
	for _, line := range contentLines {
		if len(line) > maxLineWidth {
			maxLineWidth = len(line)
		}
	}
	h.modalWidth = maxLineWidth + 6
	if h.modalWidth > HelpModalMaxWidth {
		h.modalWidth = HelpModalMaxWidth
	}
	if h.modalWidth < HelpModalMinWidth {
		h.modalWidth = HelpModalMinWidth
	}
	if h.modalWidth > h.width-2 {
		h.modalWidth = h.width - 2
	}
	if h.modalWidth < 1 {
		h.modalWidth = 1
	}
}

func (h *HelpModal) Show() {
	h.visible = true
	h.scrollOffset = 0
}

func (h *HelpModal) Hide() {
	h.visible = false
}

func (h *HelpModal) IsVisible() bool {
	return h.visible
}

func (h *HelpModal) Toggle() {
	if h.visible {
		h.Hide()
	} else {
		h.Show()
	}
}

func (h *HelpModal) ContentHeight() int {
	return h.modalHeight - 4
}

func (h *HelpModal) ContentWidth() int {
	return h.modalWidth - 4
}

func (h *HelpModal) ScrollDown() {
	h.scrollOffset++
}

func (h *HelpModal) ScrollUp() {
	if h.scrollOffset > 0 {
		h.scrollOffset--
	}
}

func (h *HelpModal) ScrollToTop() {
	h.scrollOffset = 0
}

func (h *HelpModal) ScrollToBottom() {
	contentLines := strings.Split(helpContent, "\n")
	maxOffset := len(contentLines) - h.ContentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	h.scrollOffset = maxOffset
}

func (h *HelpModal) Render(background string) string {
	if !h.visible || h.width <= 0 || h.height <= 0 {
		return background
	}

	dimmedBg := h.dimBackground(background)
	modalBox := h.renderModalBox()
	return h.overlayModal(dimmedBg, modalBox)
}

func (h *HelpModal) dimBackground(background string) string {
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	lines := strings.Split(background, "\n")
	var dimmed []string
	for _, line := range lines {
		stripped := stripAnsi(line)
		dimmed = append(dimmed, dimStyle.Render(stripped))
	}
	return strings.Join(dimmed, "\n")
}

func (h *HelpModal) renderModalBox() string {
	titleBarStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorTitleBarFg).
		Background(colorTitleBarBg).
		Width(h.modalWidth-2).
		Padding(0, 1)

	titleBar := titleBarStyle.Render("Help")

	contentLines := h.renderContent()

	innerContent := titleBar + "\n" + contentLines

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorHighlight).
		Width(h.modalWidth-2).
		Height(h.modalHeight-2).
		Padding(0, 1)

	return boxStyle.Render(innerContent)
}

func (h *HelpModal) renderContent() string {
	contentWidth := h.ContentWidth()
	contentHeight := h.ContentHeight()

	lines := strings.Split(helpContent, "\n")
	totalLines := len(lines)

	scrollOffset := h.scrollOffset
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
		if len(line) > contentWidth {
			line = line[:contentWidth-3] + "..."
		}
		rendered = append(rendered, line)
	}

	for len(rendered) < contentHeight {
		rendered = append(rendered, "")
	}

	return strings.Join(rendered, "\n")
}

func (h *HelpModal) overlayModal(background, modal string) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	for len(bgLines) < h.height {
		bgLines = append(bgLines, strings.Repeat(" ", h.width))
	}

	startY := (h.height - h.modalHeight) / 2
	startX := (h.width - h.modalWidth) / 2

	for i, modalLine := range modalLines {
		bgIdx := startY + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgIdx]
		bgRunes := []rune(stripAnsi(bgLine))

		for len(bgRunes) < h.width {
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
		if endX < h.width {
			newLine += dimStyle.Render(string(bgRunes[endX:]))
		}

		bgLines[bgIdx] = newLine
	}

	return strings.Join(bgLines, "\n")
}

func (h *HelpModal) NeedsScrolling() bool {
	lines := strings.Split(helpContent, "\n")
	return len(lines) > h.ContentHeight()
}
