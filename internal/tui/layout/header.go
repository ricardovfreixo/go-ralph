package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const HeaderHeight = 3

type HeaderData struct {
	Version      string
	Title        string
	AutoMode     bool
	Total        int
	Completed    int
	Running      int
	Failed       int
	Pending      int
	TokenUsage   string
	TotalCost    string
	ShowCost     bool
	BudgetStatus string
	BudgetAlert  bool
	ElapsedTime  string
}

type Header struct {
	width int
}

func NewHeader() *Header {
	return &Header{}
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

func (h *Header) Height() int {
	return HeaderHeight
}

func (h *Header) Render(data HeaderData) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorTitle)

	subtleStyle := lipgloss.NewStyle().
		Foreground(colorSubtle)

	autoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAuto)

	left := titleStyle.Render(data.Version)
	if data.AutoMode {
		left += " " + autoStyle.Render("[AUTO]")
	}

	summary := h.buildSummary(data)
	right := subtleStyle.Render(summary)

	topLine := h.buildTopLine(left, right)

	titleLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorHighlight).
		Render(data.Title)

	content := topLine + "\n" + titleLine

	boxStyle := lipgloss.NewStyle().
		Width(h.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorBorder).
		Padding(0, 1)

	return boxStyle.Render(content)
}

func (h *Header) buildSummary(data HeaderData) string {
	parts := []string{fmt.Sprintf("%d/%d done", data.Completed, data.Total)}

	if data.Running > 0 {
		parts = append(parts, fmt.Sprintf("%d running", data.Running))
	}
	if data.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", data.Failed))
	}
	if data.Pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", data.Pending))
	}

	summary := "[" + strings.Join(parts, ", ") + "]"

	// Show budget status if available, otherwise show cost/tokens
	if data.BudgetStatus != "" {
		budgetStyle := lipgloss.NewStyle().Foreground(colorSubtle)
		if data.BudgetAlert {
			budgetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		}
		summary += " " + budgetStyle.Render(data.BudgetStatus)
	} else if data.ShowCost && data.TotalCost != "" {
		summary += " " + data.TotalCost
	} else if data.TokenUsage != "" {
		summary += " " + data.TokenUsage
	}

	if data.ElapsedTime != "" {
		summary += " " + data.ElapsedTime
	}
	return summary
}

func (h *Header) buildTopLine(left, right string) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	contentWidth := h.width - 4

	if contentWidth <= 0 {
		return left
	}

	spaces := contentWidth - leftWidth - rightWidth
	if spaces < 1 {
		spaces = 1
	}

	return left + strings.Repeat(" ", spaces) + right
}
