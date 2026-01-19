package layout

import (
	"strings"
	"testing"
)

func TestLayoutSmallTerminal(t *testing.T) {
	l := New()
	l.SetSize(40, 10)

	headerData := HeaderData{
		Version:   AppName + " " + AppVersion,
		Title:     "Test",
		Total:     3,
		Completed: 1,
	}

	footerData := FooterData{
		Keybindings: "q: quit",
	}

	content := "Line 1\nLine 2"
	result := l.Render(headerData, footerData, content)

	if result == "" {
		t.Error("Layout should render in small terminal")
	}
	if l.ContentHeight() < 1 {
		t.Error("Content height should be at least 1 in small terminal")
	}
}

func TestLayoutMediumTerminal(t *testing.T) {
	l := New()
	l.SetSize(80, 24)

	headerData := HeaderData{
		Version:   AppName + " " + AppVersion,
		Title:     "Test PRD Title",
		Total:     5,
		Completed: 2,
		Running:   1,
		Failed:    0,
		Pending:   2,
	}

	footerData := FooterData{
		Keybindings: "s: start | S: start all | r: retry | x: stop | q: quit",
	}

	content := "Feature 1\nFeature 2\nFeature 3\nFeature 4\nFeature 5"
	result := l.Render(headerData, footerData, content)

	if !strings.Contains(result, "RALPH") {
		t.Error("Layout should contain app name")
	}
	if !strings.Contains(result, AppVersion) {
		t.Error("Layout should contain version")
	}
}

func TestLayoutLargeTerminal(t *testing.T) {
	l := New()
	l.SetSize(200, 60)

	headerData := HeaderData{
		Version:   AppName + " " + AppVersion,
		Title:     "A Very Long PRD Title That Might Need Truncation on Smaller Terminals",
		Total:     10,
		Completed: 5,
		Running:   2,
		Failed:    1,
		Pending:   2,
	}

	footerData := FooterData{
		Keybindings: "s: start | S: start all | r: retry | R: reset | x: stop | X: stop all | ?: help | q: quit",
	}

	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "Feature line")
	}
	content := strings.Join(lines, "\n")

	result := l.Render(headerData, footerData, content)

	if result == "" {
		t.Error("Layout should render in large terminal")
	}
	if l.ContentHeight() <= 40 {
		t.Error("Content height should be substantial in large terminal")
	}
}

func TestSplitPaneSmallTerminal(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(40, 10)

	leftContent := "Task 1\nTask 2\nTask 3"
	rightContent := "Activity 1\nActivity 2"

	result := sp.Render(leftContent, rightContent)

	if result == "" {
		t.Error("SplitPane should render in small terminal")
	}
	if sp.LeftPaneWidth() < MinPaneWidth {
		t.Errorf("Left pane width %d is less than minimum %d", sp.LeftPaneWidth(), MinPaneWidth)
	}
}

func TestSplitPaneLargeTerminal(t *testing.T) {
	sp := NewSplitPane()
	sp.SetSize(200, 50)

	leftContent := "A very long task name that might need truncation"
	rightContent := "Activity with timestamp and details"

	result := sp.Render(leftContent, rightContent)

	if result == "" {
		t.Error("SplitPane should render in large terminal")
	}
	if sp.LeftPaneWidth() < 90 {
		t.Error("Left pane should have more width in large terminal")
	}
}

func TestTaskListVerySmallTerminal(t *testing.T) {
	tl := NewTaskList()
	tl.SetSize(20, 3)

	items := []TaskItem{
		{ID: "1", Title: "Very Long Feature Title That Needs Truncation", Status: "pending"},
		{ID: "2", Title: "Short", Status: "running"},
	}
	tl.SetItems(items)
	tl.SetSelected(0)

	result := tl.Render()

	if result == "" {
		t.Error("TaskList should render in very small terminal")
	}
}

func TestModalSmallTerminal(t *testing.T) {
	m := NewModal()
	m.SetSize(50, 15)
	m.SetTitle("Feature Name")
	m.SetStatus("running")
	m.SetContent("Output line 1\nOutput line 2")

	background := "Background content"
	result := m.Render(background)

	if result == "" {
		t.Error("Modal should render in small terminal")
	}
}

func TestModalLargeTerminal(t *testing.T) {
	m := NewModal()
	m.SetSize(200, 60)
	m.SetTitle("A Long Feature Name")
	m.SetStatus("completed")

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "Output line")
	}
	m.SetContent(strings.Join(lines, "\n"))

	background := "Background content"
	result := m.Render(background)

	if result == "" {
		t.Error("Modal should render in large terminal")
	}
	if m.ContentHeight() <= 20 {
		t.Error("Modal content height should be substantial in large terminal")
	}
}

func TestActivityPaneSmallTerminal(t *testing.T) {
	log := NewActivityLog()
	log.AddPRDLoaded("Test PRD")
	log.AddFeatureStarted("1", "Feature 1")

	pane := NewActivityPane(log)
	pane.SetSize(30, 5)

	result := pane.Render()

	if result == "" {
		t.Error("ActivityPane should render in small terminal")
	}
}

func TestConfirmDialogSmallTerminal(t *testing.T) {
	cd := NewConfirmDialog()
	cd.SetSize(50, 15)
	cd.Show(ConfirmTypeQuit)

	background := "Background content"
	result := cd.Render(background)

	if result == "" {
		t.Error("ConfirmDialog should render in small terminal")
	}
}

func TestHelpModalSmallTerminal(t *testing.T) {
	hm := NewHelpModal()
	hm.SetSize(60, 20)
	hm.Show()

	background := "Background content"
	result := hm.Render(background)

	if result == "" {
		t.Error("HelpModal should render in small terminal")
	}
}
