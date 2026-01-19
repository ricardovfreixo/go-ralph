package layout

import "github.com/charmbracelet/lipgloss"

const (
	AppName    = "RALPH"
	AppVersion = "v0.4.0"
)

var (
	colorBorder      = lipgloss.AdaptiveColor{Light: "240", Dark: "240"}
	colorTitle       = lipgloss.AdaptiveColor{Light: "125", Dark: "205"}
	colorSubtle      = lipgloss.AdaptiveColor{Light: "245", Dark: "244"}
	colorNormal      = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	colorHighlight   = lipgloss.AdaptiveColor{Light: "27", Dark: "39"}
	colorAuto        = lipgloss.AdaptiveColor{Light: "28", Dark: "46"}
	colorRunning     = lipgloss.AdaptiveColor{Light: "178", Dark: "226"}
	colorCompleted   = lipgloss.AdaptiveColor{Light: "28", Dark: "46"}
	colorFailed      = lipgloss.AdaptiveColor{Light: "160", Dark: "196"}
	colorStopped     = lipgloss.AdaptiveColor{Light: "166", Dark: "208"}
	colorPending     = lipgloss.AdaptiveColor{Light: "245", Dark: "244"}
	colorTitleBarBg  = lipgloss.AdaptiveColor{Light: "254", Dark: "236"}
	colorTitleBarFg  = lipgloss.AdaptiveColor{Light: "235", Dark: "255"}
	colorDim         = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	colorPaneFocus   = lipgloss.AdaptiveColor{Light: "27", Dark: "39"}
	colorPaneUnfocus = lipgloss.AdaptiveColor{Light: "248", Dark: "240"}
	colorSelected    = lipgloss.AdaptiveColor{Light: "57", Dark: "57"}
	colorSelectedFg  = lipgloss.AdaptiveColor{Light: "231", Dark: "229"}
)

func StatusColor(status string) lipgloss.TerminalColor {
	switch status {
	case "running":
		return colorRunning
	case "completed":
		return colorCompleted
	case "failed":
		return colorFailed
	case "stopped":
		return colorStopped
	default:
		return colorPending
	}
}
