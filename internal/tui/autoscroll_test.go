package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/state"
	"github.com/vx/ralph-go/internal/tui/layout"
)

func mockPRD() *parser.PRD {
	return &parser.PRD{
		Title:   "Test PRD",
		Context: "Test context",
		Features: []parser.Feature{
			{
				ID:    "test-feature-1",
				Title: "Test Feature 1",
			},
		},
	}
}

func mockState() *state.Progress {
	return state.NewProgress()
}

func TestAutoScrollEnabledOnEnterInspectView(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.width = 100
	m.height = 50
	m.layout.SetSize(100, 50)
	m.splitPane.SetSize(m.layout.ContentWidth(), m.layout.ContentHeight())
	m.taskList.SetSize(m.splitPane.LeftPaneWidth(), m.splitPane.ContentHeight())
	m.modal.SetSize(100, 50)

	// Populate task list with items
	m.taskList.SetItems([]layout.TaskItem{{ID: "test-feature-1", Title: "Test Feature 1", Status: "pending"}})

	m.currentView = viewMain
	m.autoScroll = false

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
	resultModel := newModel.(Model)

	if !resultModel.autoScroll {
		t.Error("autoScroll should be enabled when entering inspect view")
	}
	if resultModel.currentView != viewInspect {
		t.Error("should switch to inspect view")
	}
	if resultModel.scrollOffset != 999999 {
		t.Errorf("scrollOffset should be set to max (999999), got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollDisabledOnScrollUp(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = true
	m.scrollOffset = 10

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	resultModel := newModel.(Model)

	if resultModel.autoScroll {
		t.Error("autoScroll should be disabled when scrolling up with 'k'")
	}
	if resultModel.scrollOffset != 9 {
		t.Errorf("scrollOffset should be 9, got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollDisabledOnGoToTop(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = true
	m.scrollOffset = 50

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	resultModel := newModel.(Model)

	if resultModel.autoScroll {
		t.Error("autoScroll should be disabled when pressing 'g' (go to top)")
	}
	if resultModel.scrollOffset != 0 {
		t.Errorf("scrollOffset should be 0, got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollReEnabledOnGoToEnd(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = false
	m.scrollOffset = 10

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	resultModel := newModel.(Model)

	if !resultModel.autoScroll {
		t.Error("autoScroll should be enabled when pressing 'G' (go to end)")
	}
	if resultModel.scrollOffset != 999999 {
		t.Errorf("scrollOffset should be max (999999), got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollReEnabledOnFollow(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = false
	m.scrollOffset = 10

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	resultModel := newModel.(Model)

	if !resultModel.autoScroll {
		t.Error("autoScroll should be enabled when pressing 'f' (follow)")
	}
	if resultModel.scrollOffset != 999999 {
		t.Errorf("scrollOffset should be max (999999), got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollResetOnExitInspectView(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = true
	m.scrollOffset = 50
	m.inspecting = "test-id"

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	resultModel := newModel.(Model)

	if resultModel.autoScroll {
		t.Error("autoScroll should be disabled when exiting inspect view")
	}
	if resultModel.scrollOffset != 0 {
		t.Errorf("scrollOffset should be reset to 0, got %d", resultModel.scrollOffset)
	}
	if resultModel.currentView != viewMain {
		t.Error("should switch back to main view")
	}
}

func TestAutoScrollNotDisabledOnScrollDown(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = true
	m.scrollOffset = 10

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	resultModel := newModel.(Model)

	if !resultModel.autoScroll {
		t.Error("autoScroll should remain enabled when scrolling down")
	}
	if resultModel.scrollOffset != 11 {
		t.Errorf("scrollOffset should be 11, got %d", resultModel.scrollOffset)
	}
}

func TestAutoScrollNotDisabledWhenAlreadyAtTopScrollUp(t *testing.T) {
	m := initialModel("test.md")
	m.prd = mockPRD()
	m.state = mockState()
	m.currentView = viewInspect
	m.autoScroll = true
	m.scrollOffset = 0

	newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	resultModel := newModel.(Model)

	if !resultModel.autoScroll {
		t.Error("autoScroll should remain enabled when at top and pressing scroll up")
	}
	if resultModel.scrollOffset != 0 {
		t.Errorf("scrollOffset should stay at 0, got %d", resultModel.scrollOffset)
	}
}
