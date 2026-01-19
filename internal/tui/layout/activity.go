package layout

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	MaxActivityEntries = 100
)

type ActivityType int

const (
	ActivityPRDLoaded ActivityType = iota
	ActivityFeatureStarted
	ActivityFeatureCompleted
	ActivityFeatureFailed
	ActivityFeatureStopped
	ActivityFeatureRetry
	ActivityOutput
)

type Activity struct {
	Type      ActivityType
	Timestamp time.Time
	Message   string
	FeatureID string
}

type ActivityLog struct {
	mu         sync.RWMutex
	entries    []Activity
	maxEntries int
}

func NewActivityLog() *ActivityLog {
	return &ActivityLog{
		entries:    make([]Activity, 0, MaxActivityEntries),
		maxEntries: MaxActivityEntries,
	}
}

func (a *ActivityLog) Add(actType ActivityType, message string, featureID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := Activity{
		Type:      actType,
		Timestamp: time.Now(),
		Message:   message,
		FeatureID: featureID,
	}

	a.entries = append([]Activity{entry}, a.entries...)

	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[:a.maxEntries]
	}
}

func (a *ActivityLog) AddPRDLoaded(title string) {
	a.Add(ActivityPRDLoaded, fmt.Sprintf("Loaded PRD: %s", title), "")
}

func (a *ActivityLog) AddFeatureStarted(featureID, title string) {
	a.Add(ActivityFeatureStarted, fmt.Sprintf("Started: %s", title), featureID)
}

func (a *ActivityLog) AddFeatureCompleted(featureID, title string) {
	a.Add(ActivityFeatureCompleted, fmt.Sprintf("Completed: %s", title), featureID)
}

func (a *ActivityLog) AddFeatureFailed(featureID, title string) {
	a.Add(ActivityFeatureFailed, fmt.Sprintf("Failed: %s", title), featureID)
}

func (a *ActivityLog) AddFeatureStopped(featureID, title string) {
	a.Add(ActivityFeatureStopped, fmt.Sprintf("Stopped: %s", title), featureID)
}

func (a *ActivityLog) AddFeatureRetry(featureID, title string, attempt int) {
	a.Add(ActivityFeatureRetry, fmt.Sprintf("Retry #%d: %s", attempt, title), featureID)
}

func (a *ActivityLog) AddOutput(featureID, message string) {
	a.Add(ActivityOutput, message, featureID)
}

func (a *ActivityLog) GetEntries() []Activity {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]Activity, len(a.entries))
	copy(result, a.entries)
	return result
}

func (a *ActivityLog) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

func (a *ActivityLog) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = make([]Activity, 0, a.maxEntries)
}

func (act Activity) FormatTimestamp() string {
	return act.Timestamp.Format("15:04")
}

func (act Activity) Color() lipgloss.TerminalColor {
	switch act.Type {
	case ActivityFeatureCompleted:
		return colorCompleted
	case ActivityFeatureFailed:
		return colorFailed
	case ActivityFeatureStarted, ActivityFeatureRetry:
		return colorRunning
	case ActivityFeatureStopped:
		return colorStopped
	case ActivityPRDLoaded, ActivityOutput:
		return colorNormal
	default:
		return colorSubtle
	}
}

func (act Activity) Render(width int) string {
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	msgStyle := lipgloss.NewStyle().Foreground(act.Color())

	timestamp := timeStyle.Render(fmt.Sprintf("[%s]", act.FormatTimestamp()))
	message := msgStyle.Render(act.Message)

	line := fmt.Sprintf("%s %s", timestamp, message)

	if lipgloss.Width(line) > width && width > 10 {
		availableWidth := width - lipgloss.Width(timestamp) - 4
		if availableWidth > 0 {
			truncatedMsg := truncateLine(act.Message, availableWidth)
			message = msgStyle.Render(truncatedMsg)
			line = fmt.Sprintf("%s %s", timestamp, message)
		}
	}

	return line
}

type ActivityPane struct {
	log          *ActivityLog
	width        int
	height       int
	scrollOffset int
}

func NewActivityPane(log *ActivityLog) *ActivityPane {
	return &ActivityPane{
		log:          log,
		scrollOffset: 0,
	}
}

func (p *ActivityPane) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *ActivityPane) ScrollUp() {
	if p.scrollOffset > 0 {
		p.scrollOffset--
	}
}

func (p *ActivityPane) ScrollDown() {
	entries := p.log.Count()
	maxOffset := entries - p.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.scrollOffset < maxOffset {
		p.scrollOffset++
	}
}

func (p *ActivityPane) ScrollToTop() {
	p.scrollOffset = 0
}

func (p *ActivityPane) ScrollToBottom() {
	entries := p.log.Count()
	maxOffset := entries - p.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	p.scrollOffset = maxOffset
}

func (p *ActivityPane) Render() string {
	entries := p.log.GetEntries()
	if len(entries) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(colorSubtle).Italic(true)
		return emptyStyle.Render("No activity yet")
	}

	start := p.scrollOffset
	end := start + p.height
	if end > len(entries) {
		end = len(entries)
	}
	if start >= len(entries) {
		start = 0
		end = p.height
		if end > len(entries) {
			end = len(entries)
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, entries[i].Render(p.width))
	}

	return joinLines(lines)
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}
