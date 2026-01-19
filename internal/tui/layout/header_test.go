package layout

import (
	"strings"
	"testing"
)

func TestHeaderNewHeader(t *testing.T) {
	h := NewHeader()
	if h == nil {
		t.Fatal("NewHeader returned nil")
	}
}

func TestHeaderSetWidthMethod(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)
	if h.width != 80 {
		t.Errorf("expected width=80, got %d", h.width)
	}
}

func TestHeaderHeightConstant(t *testing.T) {
	h := NewHeader()
	if h.Height() != HeaderHeight {
		t.Errorf("expected height=%d, got %d", HeaderHeight, h.Height())
	}
}

func TestHeaderRenderWithAllFields(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:   "ralph v0.3.0",
		Title:     "Feature Builder",
		AutoMode:  false,
		Total:     5,
		Completed: 2,
		Running:   1,
		Failed:    0,
		Pending:   2,
	}

	result := h.Render(data)

	if !strings.Contains(result, "ralph v0.3.0") {
		t.Error("should contain version")
	}
	if !strings.Contains(result, "Feature Builder") {
		t.Error("should contain title")
	}
	if !strings.Contains(result, "2/5 done") {
		t.Error("should contain completion summary")
	}
	if !strings.Contains(result, "1 running") {
		t.Error("should contain running count")
	}
	if !strings.Contains(result, "2 pending") {
		t.Error("should contain pending count")
	}
}

func TestHeaderRenderWithAutoMode(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:  "ralph v0.3.0",
		Title:    "Feature Builder",
		AutoMode: true,
		Total:    5,
	}

	result := h.Render(data)

	if !strings.Contains(result, "[AUTO]") {
		t.Error("should contain [AUTO] when auto mode is enabled")
	}
}

func TestHeaderRenderWithoutAutoMode(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:  "ralph v0.3.0",
		Title:    "Feature Builder",
		AutoMode: false,
		Total:    5,
	}

	result := h.Render(data)

	if strings.Contains(result, "[AUTO]") {
		t.Error("should not contain [AUTO] when auto mode is disabled")
	}
}

func TestHeaderRenderWithTokenUsage(t *testing.T) {
	h := NewHeader()
	h.SetWidth(100)

	data := HeaderData{
		Version:    "ralph v0.3.0",
		Title:      "Feature Builder",
		Total:      5,
		Completed:  2,
		TokenUsage: "5.2k↓ 1.8k↑",
	}

	result := h.Render(data)

	if !strings.Contains(result, "5.2k↓ 1.8k↑") {
		t.Error("should contain token usage when provided")
	}
}

func TestHeaderRenderNoTokenUsage(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	data := HeaderData{
		Version:    "ralph v0.3.0",
		Title:      "Feature Builder",
		Total:      5,
		TokenUsage: "",
	}

	result := h.Render(data)

	if strings.Contains(result, "↓") || strings.Contains(result, "↑") {
		t.Error("should not contain token arrows when no usage provided")
	}
}

func TestBuildSummary(t *testing.T) {
	h := NewHeader()

	data := HeaderData{
		Total:      10,
		Completed:  5,
		Running:    2,
		Failed:     1,
		Pending:    2,
		TokenUsage: "1.5k↓ 500↑",
	}

	summary := h.buildSummary(data)

	if !strings.Contains(summary, "5/10 done") {
		t.Error("should contain completion ratio")
	}
	if !strings.Contains(summary, "2 running") {
		t.Error("should contain running count")
	}
	if !strings.Contains(summary, "1 failed") {
		t.Error("should contain failed count")
	}
	if !strings.Contains(summary, "2 pending") {
		t.Error("should contain pending count")
	}
	if !strings.Contains(summary, "1.5k↓ 500↑") {
		t.Error("should contain token usage")
	}
}

func TestBuildSummaryNoOptionalCounts(t *testing.T) {
	h := NewHeader()

	data := HeaderData{
		Total:     5,
		Completed: 5,
		Running:   0,
		Failed:    0,
		Pending:   0,
	}

	summary := h.buildSummary(data)

	if strings.Contains(summary, "running") {
		t.Error("should not contain running when count is 0")
	}
	if strings.Contains(summary, "failed") {
		t.Error("should not contain failed when count is 0")
	}
	if strings.Contains(summary, "pending") {
		t.Error("should not contain pending when count is 0")
	}
}

func TestBuildTopLine(t *testing.T) {
	h := NewHeader()
	h.SetWidth(80)

	left := "ralph"
	right := "[done]"

	topLine := h.buildTopLine(left, right)

	if !strings.HasPrefix(topLine, left) {
		t.Error("top line should start with left content")
	}
	if !strings.HasSuffix(topLine, right) {
		t.Error("top line should end with right content")
	}
}
