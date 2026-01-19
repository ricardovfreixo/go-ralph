package layout

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestActivityLogAdd(t *testing.T) {
	log := NewActivityLog()

	log.Add(ActivityFeatureStarted, "Test message", "feature-123")

	entries := log.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", entries[0].Message)
	}
	if entries[0].FeatureID != "feature-123" {
		t.Errorf("Expected featureID 'feature-123', got '%s'", entries[0].FeatureID)
	}
	if entries[0].Type != ActivityFeatureStarted {
		t.Errorf("Expected type ActivityFeatureStarted, got %v", entries[0].Type)
	}
}

func TestActivityLogReverseChronological(t *testing.T) {
	log := NewActivityLog()

	log.Add(ActivityFeatureStarted, "First", "")
	time.Sleep(10 * time.Millisecond)
	log.Add(ActivityFeatureCompleted, "Second", "")
	time.Sleep(10 * time.Millisecond)
	log.Add(ActivityFeatureFailed, "Third", "")

	entries := log.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	if entries[0].Message != "Third" {
		t.Errorf("Expected first entry to be 'Third' (newest), got '%s'", entries[0].Message)
	}
	if entries[1].Message != "Second" {
		t.Errorf("Expected second entry to be 'Second', got '%s'", entries[1].Message)
	}
	if entries[2].Message != "First" {
		t.Errorf("Expected third entry to be 'First' (oldest), got '%s'", entries[2].Message)
	}
}

func TestActivityLogMaxEntries(t *testing.T) {
	log := NewActivityLog()

	for i := 0; i < 150; i++ {
		log.Add(ActivityFeatureStarted, "Message", "")
	}

	entries := log.GetEntries()
	if len(entries) != MaxActivityEntries {
		t.Errorf("Expected %d entries (max), got %d", MaxActivityEntries, len(entries))
	}
}

func TestActivityLogHelperMethods(t *testing.T) {
	log := NewActivityLog()

	log.AddPRDLoaded("Test PRD")
	entries := log.GetEntries()
	if !strings.Contains(entries[0].Message, "Test PRD") {
		t.Errorf("AddPRDLoaded should include PRD title")
	}
	if entries[0].Type != ActivityPRDLoaded {
		t.Errorf("AddPRDLoaded should set type to ActivityPRDLoaded")
	}

	log.AddFeatureStarted("feat-1", "Feature 1")
	entries = log.GetEntries()
	if !strings.Contains(entries[0].Message, "Feature 1") {
		t.Errorf("AddFeatureStarted should include feature title")
	}
	if entries[0].Type != ActivityFeatureStarted {
		t.Errorf("AddFeatureStarted should set type to ActivityFeatureStarted")
	}

	log.AddFeatureCompleted("feat-2", "Feature 2")
	entries = log.GetEntries()
	if !strings.Contains(entries[0].Message, "Feature 2") {
		t.Errorf("AddFeatureCompleted should include feature title")
	}
	if entries[0].Type != ActivityFeatureCompleted {
		t.Errorf("AddFeatureCompleted should set type to ActivityFeatureCompleted")
	}

	log.AddFeatureFailed("feat-3", "Feature 3")
	entries = log.GetEntries()
	if !strings.Contains(entries[0].Message, "Feature 3") {
		t.Errorf("AddFeatureFailed should include feature title")
	}
	if entries[0].Type != ActivityFeatureFailed {
		t.Errorf("AddFeatureFailed should set type to ActivityFeatureFailed")
	}

	log.AddFeatureStopped("feat-4", "Feature 4")
	entries = log.GetEntries()
	if !strings.Contains(entries[0].Message, "Feature 4") {
		t.Errorf("AddFeatureStopped should include feature title")
	}
	if entries[0].Type != ActivityFeatureStopped {
		t.Errorf("AddFeatureStopped should set type to ActivityFeatureStopped")
	}

	log.AddFeatureRetry("feat-5", "Feature 5", 2)
	entries = log.GetEntries()
	if !strings.Contains(entries[0].Message, "Feature 5") {
		t.Errorf("AddFeatureRetry should include feature title")
	}
	if !strings.Contains(entries[0].Message, "#2") {
		t.Errorf("AddFeatureRetry should include attempt number")
	}
	if entries[0].Type != ActivityFeatureRetry {
		t.Errorf("AddFeatureRetry should set type to ActivityFeatureRetry")
	}
}

func TestActivityTimestampFormat(t *testing.T) {
	act := Activity{
		Timestamp: time.Date(2026, 1, 19, 14, 30, 0, 0, time.UTC),
	}

	formatted := act.FormatTimestamp()
	if formatted != "14:30" {
		t.Errorf("Expected timestamp format '14:30', got '%s'", formatted)
	}
}

func TestActivityColor(t *testing.T) {
	tests := []struct {
		actType  ActivityType
		expected lipgloss.TerminalColor
	}{
		{ActivityFeatureCompleted, colorCompleted},
		{ActivityFeatureFailed, colorFailed},
		{ActivityFeatureStarted, colorRunning},
		{ActivityFeatureRetry, colorRunning},
		{ActivityFeatureStopped, colorStopped},
		{ActivityPRDLoaded, colorNormal},
		{ActivityOutput, colorNormal},
	}

	for _, tt := range tests {
		act := Activity{Type: tt.actType}
		color := act.Color()
		if color != tt.expected {
			t.Errorf("Activity type %v: colors don't match", tt.actType)
		}
	}
}

func TestActivityRender(t *testing.T) {
	act := Activity{
		Type:      ActivityFeatureStarted,
		Timestamp: time.Date(2026, 1, 19, 12, 34, 0, 0, time.UTC),
		Message:   "Started: Test Feature",
	}

	rendered := act.Render(50)

	if !strings.Contains(rendered, "12:34") {
		t.Errorf("Rendered activity should contain timestamp")
	}
	if !strings.Contains(rendered, "Started: Test Feature") {
		t.Errorf("Rendered activity should contain message")
	}
}

func TestActivityLogCount(t *testing.T) {
	log := NewActivityLog()

	if log.Count() != 0 {
		t.Errorf("New log should have count 0")
	}

	log.Add(ActivityFeatureStarted, "Test", "")
	if log.Count() != 1 {
		t.Errorf("After adding one entry, count should be 1")
	}
}

func TestActivityLogClear(t *testing.T) {
	log := NewActivityLog()

	log.Add(ActivityFeatureStarted, "Test 1", "")
	log.Add(ActivityFeatureStarted, "Test 2", "")
	log.Clear()

	if log.Count() != 0 {
		t.Errorf("After Clear(), count should be 0, got %d", log.Count())
	}
}

func TestActivityPaneSetSize(t *testing.T) {
	log := NewActivityLog()
	pane := NewActivityPane(log)

	pane.SetSize(50, 20)

	if pane.width != 50 || pane.height != 20 {
		t.Errorf("SetSize should update width and height")
	}
}

func TestActivityPaneScrolling(t *testing.T) {
	log := NewActivityLog()
	for i := 0; i < 30; i++ {
		log.Add(ActivityFeatureStarted, "Test", "")
	}

	pane := NewActivityPane(log)
	pane.SetSize(50, 10)

	if pane.scrollOffset != 0 {
		t.Errorf("Initial scroll offset should be 0")
	}

	pane.ScrollDown()
	if pane.scrollOffset != 1 {
		t.Errorf("After ScrollDown, offset should be 1")
	}

	pane.ScrollUp()
	if pane.scrollOffset != 0 {
		t.Errorf("After ScrollUp from 1, offset should be 0")
	}

	pane.ScrollUp()
	if pane.scrollOffset != 0 {
		t.Errorf("ScrollUp at offset 0 should stay at 0")
	}

	pane.ScrollToBottom()
	expectedOffset := log.Count() - pane.height
	if pane.scrollOffset != expectedOffset {
		t.Errorf("ScrollToBottom should set offset to %d, got %d", expectedOffset, pane.scrollOffset)
	}

	pane.ScrollToTop()
	if pane.scrollOffset != 0 {
		t.Errorf("ScrollToTop should set offset to 0")
	}
}

func TestActivityPaneRenderEmpty(t *testing.T) {
	log := NewActivityLog()
	pane := NewActivityPane(log)
	pane.SetSize(50, 10)

	rendered := pane.Render()
	if !strings.Contains(rendered, "No activity yet") {
		t.Errorf("Empty activity pane should show 'No activity yet'")
	}
}

func TestActivityPaneRenderWithEntries(t *testing.T) {
	log := NewActivityLog()
	log.Add(ActivityFeatureStarted, "Test Feature", "feat-1")
	log.Add(ActivityFeatureCompleted, "Test Feature 2", "feat-2")

	pane := NewActivityPane(log)
	pane.SetSize(80, 10)

	rendered := pane.Render()
	if !strings.Contains(rendered, "Test Feature") {
		t.Errorf("Rendered pane should contain activity messages")
	}
}

func TestActivityLogThreadSafety(t *testing.T) {
	log := NewActivityLog()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				log.Add(ActivityFeatureStarted, "Test", "")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	count := log.Count()
	if count > MaxActivityEntries {
		t.Errorf("Log should respect max entries even with concurrent access")
	}
}
