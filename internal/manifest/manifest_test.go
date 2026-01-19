package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vx/ralph-go/internal/parser"
)

func TestNewManifest(t *testing.T) {
	m := New("test.md", "Test Project")

	if m.Source != "test.md" {
		t.Errorf("expected source 'test.md', got %q", m.Source)
	}
	if m.Title != "Test Project" {
		t.Errorf("expected title 'Test Project', got %q", m.Title)
	}
	if len(m.Features) != 0 {
		t.Errorf("expected empty features, got %d", len(m.Features))
	}
	if m.Created.IsZero() {
		t.Error("expected Created to be set")
	}
}

func TestManifestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")

	m := New("test.md", "Test Project")
	m.SetPath(manifestPath)
	m.Features = []ManifestFeature{
		{
			ID:        "01",
			Dir:       "01-feature-one",
			Title:     "Feature One",
			Status:    "pending",
			DependsOn: []string{},
			Execution: "sequential",
			Model:     "sonnet",
		},
		{
			ID:        "02",
			Dir:       "02-feature-two",
			Title:     "Feature Two",
			Status:    "completed",
			DependsOn: []string{"01"},
			Execution: "parallel",
			Model:     "opus",
		},
	}

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if loaded.Source != m.Source {
		t.Errorf("source mismatch: expected %q, got %q", m.Source, loaded.Source)
	}
	if loaded.Title != m.Title {
		t.Errorf("title mismatch: expected %q, got %q", m.Title, loaded.Title)
	}
	if len(loaded.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(loaded.Features))
	}
	if loaded.Features[0].ID != "01" {
		t.Errorf("expected feature ID '01', got %q", loaded.Features[0].ID)
	}
	if loaded.Features[1].DependsOn[0] != "01" {
		t.Errorf("expected dependency '01', got %q", loaded.Features[1].DependsOn[0])
	}
}

func TestUpdateFeatureStatus(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One", Status: "pending"},
	}

	err := m.UpdateFeatureStatus("01", "running")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	if m.Features[0].Status != "running" {
		t.Errorf("expected status 'running', got %q", m.Features[0].Status)
	}

	err = m.UpdateFeatureStatus("99", "running")
	if err == nil {
		t.Error("expected error for non-existent feature")
	}
}

func TestGetFeature(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One"},
		{ID: "02", Title: "Feature Two"},
	}

	f := m.GetFeature("01")
	if f == nil {
		t.Fatal("expected feature, got nil")
	}
	if f.Title != "Feature One" {
		t.Errorf("expected 'Feature One', got %q", f.Title)
	}

	f = m.GetFeature("99")
	if f != nil {
		t.Error("expected nil for non-existent feature")
	}
}

func TestGetFeatureByTitle(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One"},
		{ID: "02", Title: "Feature Two"},
	}

	f := m.GetFeatureByTitle("Feature One")
	if f == nil {
		t.Fatal("expected feature, got nil")
	}
	if f.ID != "01" {
		t.Errorf("expected ID '01', got %q", f.ID)
	}

	f = m.GetFeatureByTitle("feature one")
	if f == nil {
		t.Error("expected case-insensitive match")
	}

	f = m.GetFeatureByTitle("Non-existent")
	if f != nil {
		t.Error("expected nil for non-existent feature")
	}
}

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single numeric dependency",
			content:  "Depends: 01",
			expected: []string{"01"},
		},
		{
			name:     "multiple numeric dependencies",
			content:  "Depends: 01, 02, 03",
			expected: []string{"01", "02", "03"},
		},
		{
			name:     "title dependency",
			content:  "Depends: Layout Foundation",
			expected: []string{"Layout Foundation"},
		},
		{
			name:     "mixed dependencies",
			content:  "Depends: 01, Layout Foundation",
			expected: []string{"01", "Layout Foundation"},
		},
		{
			name:     "case insensitive",
			content:  "depends: 01",
			expected: []string{"01"},
		},
		{
			name:     "with extra whitespace",
			content:  "Depends:   01  ,   02  ",
			expected: []string{"01", "02"},
		},
		{
			name:     "multiline content",
			content:  "Some description\nDepends: 01, 02\nMore content",
			expected: []string{"01", "02"},
		},
		{
			name:     "no dependencies",
			content:  "Just a description without depends field",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParseDependencies(tt.content, "")
			if len(deps) != len(tt.expected) {
				t.Fatalf("expected %d dependencies, got %d: %v", len(tt.expected), len(deps), deps)
			}
			for i, exp := range tt.expected {
				if deps[i] != exp {
					t.Errorf("dependency %d: expected %q, got %q", i, exp, deps[i])
				}
			}
		})
	}
}

func TestResolveDependencyID(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Layout Foundation"},
		{ID: "02", Title: "Two-Pane Content"},
		{ID: "03", Title: "Feature Three"},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"01", "01"},
		{"1", "01"},
		{"Layout Foundation", "01"},
		{"layout foundation", "01"},
		{"Two-Pane Content", "02"},
		{"99", "99"},
		{"Non-existent", "Non-existent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := m.ResolveDependencyID(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestResolveDependencies(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Layout Foundation", DependsOn: []string{}},
		{ID: "02", Title: "Two-Pane Content", DependsOn: []string{"1"}},
		{ID: "03", Title: "Feature Three", DependsOn: []string{"Layout Foundation", "02"}},
	}

	m.ResolveDependencies()

	if len(m.Features[1].DependsOn) != 1 || m.Features[1].DependsOn[0] != "01" {
		t.Errorf("feature 02 dependency not resolved: %v", m.Features[1].DependsOn)
	}

	if len(m.Features[2].DependsOn) != 2 {
		t.Fatalf("expected 2 dependencies for feature 03, got %d", len(m.Features[2].DependsOn))
	}
	if m.Features[2].DependsOn[0] != "01" {
		t.Errorf("expected first dependency '01', got %q", m.Features[2].DependsOn[0])
	}
	if m.Features[2].DependsOn[1] != "02" {
		t.Errorf("expected second dependency '02', got %q", m.Features[2].DependsOn[1])
	}
}

func TestGenerateFromPRD(t *testing.T) {
	prdContent := `# Test Project

This is the project context.

## Feature 1: Layout Foundation

Implement the layout.

Execution: sequential
Model: sonnet

- [ ] Create layout component
- [ ] Add tests

Acceptance: Layout renders correctly

---

## Feature 2: Content Area

Depends: 01
Execution: parallel
Model: opus

- [ ] Create content component

Acceptance: Content displays properly

---

## Feature 3: Integration

Depends: 01, Feature 2: Content Area

- [ ] Integrate everything
`

	prd, err := parser.ParsePRDContent(prdContent)
	if err != nil {
		t.Fatalf("failed to parse PRD: %v", err)
	}

	manifest, err := GenerateFromPRD(prd, "/path/to/test.md")
	if err != nil {
		t.Fatalf("failed to generate manifest: %v", err)
	}

	if manifest.Title != "Test Project" {
		t.Errorf("expected title 'Test Project', got %q", manifest.Title)
	}
	if manifest.Source != "test.md" {
		t.Errorf("expected source 'test.md', got %q", manifest.Source)
	}

	if len(manifest.Features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(manifest.Features))
	}

	f1 := manifest.Features[0]
	if f1.ID != "01" {
		t.Errorf("feature 1: expected ID '01', got %q", f1.ID)
	}
	if f1.Execution != "sequential" {
		t.Errorf("feature 1: expected execution 'sequential', got %q", f1.Execution)
	}
	if f1.Model != "sonnet" {
		t.Errorf("feature 1: expected model 'sonnet', got %q", f1.Model)
	}
	if len(f1.DependsOn) != 0 {
		t.Errorf("feature 1: expected no dependencies, got %v", f1.DependsOn)
	}

	f2 := manifest.Features[1]
	if f2.Execution != "parallel" {
		t.Errorf("feature 2: expected execution 'parallel', got %q", f2.Execution)
	}
	if f2.Model != "opus" {
		t.Errorf("feature 2: expected model 'opus', got %q", f2.Model)
	}
	if len(f2.DependsOn) != 1 || f2.DependsOn[0] != "01" {
		t.Errorf("feature 2: expected dependency ['01'], got %v", f2.DependsOn)
	}

	f3 := manifest.Features[2]
	if len(f3.DependsOn) != 2 {
		t.Errorf("feature 3: expected 2 dependencies, got %v", f3.DependsOn)
	}
}

func TestSanitizeDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Layout Foundation", "layout-foundation"},
		{"Feature 1: My Feature", "my-feature"},
		{"Two-Pane Content Area", "two-pane-content-area"},
		{"Test With  Multiple   Spaces", "test-with-multiple-spaces"},
		{"Special!@#$Characters", "specialcharacters"},
		{"Very Long Title That Should Be Truncated Because It Exceeds The Maximum Length Allowed", "very-long-title-that-should-be-truncated-because-i"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeDirName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsDependencySatisfied(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One", Status: "completed", DependsOn: []string{}},
		{ID: "02", Title: "Feature Two", Status: "pending", DependsOn: []string{"01"}},
		{ID: "03", Title: "Feature Three", Status: "pending", DependsOn: []string{"01", "02"}},
	}

	if !m.IsDependencySatisfied("01") {
		t.Error("feature 01 should have satisfied dependencies (no deps)")
	}

	if !m.IsDependencySatisfied("02") {
		t.Error("feature 02 should have satisfied dependencies (01 completed)")
	}

	if m.IsDependencySatisfied("03") {
		t.Error("feature 03 should NOT have satisfied dependencies (02 pending)")
	}

	m.Features[1].Status = "completed"
	if !m.IsDependencySatisfied("03") {
		t.Error("feature 03 should now have satisfied dependencies")
	}
}

func TestGetPendingDependencies(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One", Status: "completed", DependsOn: []string{}},
		{ID: "02", Title: "Feature Two", Status: "pending", DependsOn: []string{}},
		{ID: "03", Title: "Feature Three", Status: "pending", DependsOn: []string{"01", "02"}},
	}

	pending := m.GetPendingDependencies("01")
	if len(pending) != 0 {
		t.Errorf("expected no pending deps for 01, got %v", pending)
	}

	pending = m.GetPendingDependencies("03")
	if len(pending) != 1 || pending[0] != "02" {
		t.Errorf("expected pending deps ['02'] for 03, got %v", pending)
	}
}

func TestGetSummary(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Status: "completed", DependsOn: []string{}},
		{ID: "02", Status: "running", DependsOn: []string{}},
		{ID: "03", Status: "failed", DependsOn: []string{}},
		{ID: "04", Status: "pending", DependsOn: []string{}},
		{ID: "05", Status: "pending", DependsOn: []string{"03"}},
	}

	total, completed, running, failed, pending, blocked := m.GetSummary()

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed 1, got %d", completed)
	}
	if running != 1 {
		t.Errorf("expected running 1, got %d", running)
	}
	if failed != 1 {
		t.Errorf("expected failed 1, got %d", failed)
	}
	if pending != 1 {
		t.Errorf("expected pending 1, got %d", pending)
	}
	if blocked != 1 {
		t.Errorf("expected blocked 1, got %d", blocked)
	}
}

func TestManifestJSONStructure(t *testing.T) {
	m := New("PRD-TUI-REVAMP.md", "Ralph TUI Revamp v0.3.0")
	m.Features = []ManifestFeature{
		{
			ID:        "01",
			Dir:       "01-layout-foundation",
			Title:     "Layout Foundation",
			Status:    "completed",
			DependsOn: []string{},
			Execution: "sequential",
			Model:     "sonnet",
		},
		{
			ID:        "02",
			Dir:       "02-two-pane-content",
			Title:     "Two-Pane Content Area",
			Status:    "pending",
			DependsOn: []string{"01"},
			Execution: "sequential",
			Model:     "sonnet",
		},
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := result["source"]; !ok {
		t.Error("missing 'source' field")
	}
	if _, ok := result["title"]; !ok {
		t.Error("missing 'title' field")
	}
	if _, ok := result["created"]; !ok {
		t.Error("missing 'created' field")
	}
	if _, ok := result["features"]; !ok {
		t.Error("missing 'features' field")
	}

	features := result["features"].([]interface{})
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}

	f1 := features[0].(map[string]interface{})
	requiredFields := []string{"id", "dir", "title", "status", "depends_on", "execution", "model"}
	for _, field := range requiredFields {
		if _, ok := f1[field]; !ok {
			t.Errorf("missing feature field: %s", field)
		}
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/non/existent/path")
	if err == nil {
		t.Error("expected error loading non-existent manifest")
	}
}

func TestAllFeatures(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One"},
		{ID: "02", Title: "Feature Two"},
	}

	features := m.AllFeatures()
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}

	features[0].Title = "Modified"
	if m.Features[0].Title == "Modified" {
		t.Error("AllFeatures should return a copy, not modify original")
	}
}

func TestParserDependsField(t *testing.T) {
	prdContent := `# Test Project

Context here.

## Feature 1: First Feature

Description of first feature.

- [ ] Task 1

---

## Feature 2: Second Feature

Depends: 01

- [ ] Task 2

---

## Feature 3: Third Feature

Depends: First Feature, 02

- [ ] Task 3
`

	prd, err := parser.ParsePRDContent(prdContent)
	if err != nil {
		t.Fatalf("failed to parse PRD: %v", err)
	}

	if len(prd.Features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(prd.Features))
	}

	if len(prd.Features[0].DependsOn) != 0 {
		t.Errorf("feature 1: expected no dependencies, got %v", prd.Features[0].DependsOn)
	}

	if len(prd.Features[1].DependsOn) != 1 || prd.Features[1].DependsOn[0] != "01" {
		t.Errorf("feature 2: expected dependency ['01'], got %v", prd.Features[1].DependsOn)
	}

	if len(prd.Features[2].DependsOn) != 2 {
		t.Fatalf("feature 3: expected 2 dependencies, got %d", len(prd.Features[2].DependsOn))
	}
	if prd.Features[2].DependsOn[0] != "First Feature" {
		t.Errorf("feature 3: expected first dependency 'First Feature', got %q", prd.Features[2].DependsOn[0])
	}
	if prd.Features[2].DependsOn[1] != "02" {
		t.Errorf("feature 3: expected second dependency '02', got %q", prd.Features[2].DependsOn[1])
	}
}

func TestParserRawContent(t *testing.T) {
	prdContent := `# Test Project

Context here.

## Feature 1: My Feature

Depends: 01
Execution: parallel
Model: opus

Some description text.

- [ ] Task 1
- [ ] Task 2

Acceptance: Feature works
`

	prd, err := parser.ParsePRDContent(prdContent)
	if err != nil {
		t.Fatalf("failed to parse PRD: %v", err)
	}

	if len(prd.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(prd.Features))
	}

	rawContent := prd.Features[0].RawContent
	if rawContent == "" {
		t.Fatal("expected non-empty raw content")
	}

	mustContain := []string{
		"## Feature 1: My Feature",
		"Depends: 01",
		"Execution: parallel",
		"Model: opus",
		"- [ ] Task 1",
	}

	for _, s := range mustContain {
		if !contains(rawContent, s) {
			t.Errorf("raw content should contain %q", s)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestManifestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")

	m := New("test.md", "Test Project")
	m.SetPath(manifestPath)
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Feature One", Status: "pending"},
		{ID: "02", Title: "Feature Two", Status: "pending"},
	}

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	if err := m.UpdateFeatureStatus("01", "completed"); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	if err := m.Save(); err != nil {
		t.Fatalf("failed to save after update: %v", err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	if loaded.Features[0].Status != "completed" {
		t.Errorf("expected status 'completed' after reload, got %q", loaded.Features[0].Status)
	}

	data, _ := os.ReadFile(manifestPath)
	t.Logf("Manifest content:\n%s", string(data))
}

// Tests for Recursive Feature Data Model (RLM support)

func TestDefaultMaxDepth(t *testing.T) {
	if DefaultMaxDepth != 3 {
		t.Errorf("expected DefaultMaxDepth to be 3, got %d", DefaultMaxDepth)
	}
}

func TestGetMaxDepth(t *testing.T) {
	m := New("test.md", "Test Project")

	if m.GetMaxDepth() != DefaultMaxDepth {
		t.Errorf("expected default max depth %d, got %d", DefaultMaxDepth, m.GetMaxDepth())
	}

	m.SetMaxDepth(5)
	if m.GetMaxDepth() != 5 {
		t.Errorf("expected max depth 5, got %d", m.GetMaxDepth())
	}

	m.SetMaxDepth(0)
	if m.GetMaxDepth() != DefaultMaxDepth {
		t.Errorf("expected default when 0 set, got %d", m.GetMaxDepth())
	}
}

func TestIsRootFeature(t *testing.T) {
	root := ManifestFeature{ID: "01", ParentID: ""}
	child := ManifestFeature{ID: "01-01", ParentID: "01"}

	if !root.IsRootFeature() {
		t.Error("expected root to be a root feature")
	}
	if child.IsRootFeature() {
		t.Error("expected child to not be a root feature")
	}
}

func TestHasChildren(t *testing.T) {
	noChildren := ManifestFeature{ID: "01", Children: nil}
	withChildren := ManifestFeature{ID: "01", Children: []string{"01-01", "01-02"}}
	emptyChildren := ManifestFeature{ID: "01", Children: []string{}}

	if noChildren.HasChildren() {
		t.Error("expected nil Children to return false")
	}
	if !withChildren.HasChildren() {
		t.Error("expected feature with children to return true")
	}
	if emptyChildren.HasChildren() {
		t.Error("expected empty Children slice to return false")
	}
}

func TestAddSubFeature(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root Feature", Depth: 0, ContextBudget: 10000},
	}

	child := ManifestFeature{
		ID:    "01-01",
		Title: "Child Feature",
	}

	err := m.AddSubFeature("01", child)
	if err != nil {
		t.Fatalf("failed to add sub-feature: %v", err)
	}

	if len(m.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(m.Features))
	}

	addedChild := m.GetFeature("01-01")
	if addedChild == nil {
		t.Fatal("child feature not found")
	}
	if addedChild.ParentID != "01" {
		t.Errorf("expected parent ID '01', got %q", addedChild.ParentID)
	}
	if addedChild.Depth != 1 {
		t.Errorf("expected depth 1, got %d", addedChild.Depth)
	}
	if addedChild.ContextBudget != 5000 {
		t.Errorf("expected context budget 5000 (half of parent), got %d", addedChild.ContextBudget)
	}

	parent := m.GetFeature("01")
	if len(parent.Children) != 1 || parent.Children[0] != "01-01" {
		t.Errorf("parent should have child '01-01', got %v", parent.Children)
	}
}

func TestAddSubFeatureParentNotFound(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root Feature"},
	}

	child := ManifestFeature{ID: "02-01", Title: "Child Feature"}
	err := m.AddSubFeature("02", child)
	if err == nil {
		t.Error("expected error when parent not found")
	}
}

func TestAddSubFeatureMaxDepthExceeded(t *testing.T) {
	m := New("test.md", "Test Project")
	m.SetMaxDepth(2)
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Depth: 0},
		{ID: "01-01", Title: "Level 1", Depth: 1, ParentID: "01"},
		{ID: "01-01-01", Title: "Level 2", Depth: 2, ParentID: "01-01"},
	}
	m.Features[0].Children = []string{"01-01"}
	m.Features[1].Children = []string{"01-01-01"}

	child := ManifestFeature{ID: "01-01-01-01", Title: "Level 3"}
	err := m.AddSubFeature("01-01-01", child)
	if err == nil {
		t.Error("expected error when max depth exceeded")
	}
}

func TestGetChildren(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Children: []string{"01-01", "01-02"}},
		{ID: "01-01", Title: "Child 1", ParentID: "01"},
		{ID: "01-02", Title: "Child 2", ParentID: "01"},
		{ID: "02", Title: "Another Root"},
	}

	children := m.GetChildren("01")
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0].ID != "01-01" || children[1].ID != "01-02" {
		t.Errorf("unexpected children: %v", children)
	}

	noChildren := m.GetChildren("02")
	if len(noChildren) != 0 {
		t.Errorf("expected no children, got %d", len(noChildren))
	}

	nilChildren := m.GetChildren("99")
	if nilChildren != nil {
		t.Error("expected nil for non-existent parent")
	}
}

func TestGetParent(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root"},
		{ID: "01-01", Title: "Child", ParentID: "01"},
	}

	parent := m.GetParent("01-01")
	if parent == nil {
		t.Fatal("expected parent, got nil")
	}
	if parent.ID != "01" {
		t.Errorf("expected parent ID '01', got %q", parent.ID)
	}

	noParent := m.GetParent("01")
	if noParent != nil {
		t.Error("expected nil for root feature")
	}

	nilParent := m.GetParent("99")
	if nilParent != nil {
		t.Error("expected nil for non-existent feature")
	}
}

func TestGetRootFeatures(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root 1"},
		{ID: "01-01", Title: "Child of 01", ParentID: "01"},
		{ID: "02", Title: "Root 2"},
		{ID: "02-01", Title: "Child of 02", ParentID: "02"},
	}

	roots := m.GetRootFeatures()
	if len(roots) != 2 {
		t.Fatalf("expected 2 root features, got %d", len(roots))
	}
	if roots[0].ID != "01" || roots[1].ID != "02" {
		t.Errorf("unexpected roots: %v", roots)
	}
}

func TestGetDescendants(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Children: []string{"01-01", "01-02"}},
		{ID: "01-01", Title: "Child 1", ParentID: "01", Children: []string{"01-01-01"}},
		{ID: "01-02", Title: "Child 2", ParentID: "01"},
		{ID: "01-01-01", Title: "Grandchild", ParentID: "01-01"},
	}

	descendants := m.GetDescendants("01")
	if len(descendants) != 3 {
		t.Fatalf("expected 3 descendants, got %d", len(descendants))
	}

	ids := make(map[string]bool)
	for _, d := range descendants {
		ids[d.ID] = true
	}
	if !ids["01-01"] || !ids["01-02"] || !ids["01-01-01"] {
		t.Errorf("missing expected descendants: %v", ids)
	}

	noDescendants := m.GetDescendants("01-02")
	if len(noDescendants) != 0 {
		t.Errorf("expected no descendants for leaf, got %d", len(noDescendants))
	}
}

func TestGetFeatureWithDescendants(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Children: []string{"01-01"}},
		{ID: "01-01", Title: "Child", ParentID: "01"},
	}

	all := m.GetFeatureWithDescendants("01")
	if len(all) != 2 {
		t.Fatalf("expected 2 features (root + child), got %d", len(all))
	}
	if all[0].ID != "01" {
		t.Errorf("first should be root, got %q", all[0].ID)
	}

	nilResult := m.GetFeatureWithDescendants("99")
	if nilResult != nil {
		t.Error("expected nil for non-existent feature")
	}
}

func TestGetAncestors(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root"},
		{ID: "01-01", Title: "Child", ParentID: "01"},
		{ID: "01-01-01", Title: "Grandchild", ParentID: "01-01"},
	}

	ancestors := m.GetAncestors("01-01-01")
	if len(ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d", len(ancestors))
	}
	if ancestors[0].ID != "01-01" || ancestors[1].ID != "01" {
		t.Errorf("unexpected ancestor order: %v", ancestors)
	}

	noAncestors := m.GetAncestors("01")
	if len(noAncestors) != 0 {
		t.Errorf("expected no ancestors for root, got %d", len(noAncestors))
	}

	nilAncestors := m.GetAncestors("99")
	if nilAncestors != nil {
		t.Error("expected nil for non-existent feature")
	}
}

func TestCanSpawnChild(t *testing.T) {
	m := New("test.md", "Test Project")
	m.SetMaxDepth(2)
	m.Features = []ManifestFeature{
		{ID: "01", Depth: 0},
		{ID: "01-01", Depth: 1, ParentID: "01"},
		{ID: "01-01-01", Depth: 2, ParentID: "01-01"},
	}

	if !m.CanSpawnChild("01") {
		t.Error("depth 0 should be able to spawn (max 2)")
	}
	if !m.CanSpawnChild("01-01") {
		t.Error("depth 1 should be able to spawn (max 2)")
	}
	if m.CanSpawnChild("01-01-01") {
		t.Error("depth 2 should NOT be able to spawn (max 2)")
	}
	if m.CanSpawnChild("99") {
		t.Error("non-existent feature should return false")
	}
}

func TestRecursiveFeatureJSONSerialization(t *testing.T) {
	m := New("test.md", "Test Project")
	m.SetMaxDepth(5)
	m.Features = []ManifestFeature{
		{
			ID:            "01",
			Title:         "Root Feature",
			Status:        "running",
			Children:      []string{"01-01"},
			Depth:         0,
			ContextBudget: 10000,
		},
		{
			ID:            "01-01",
			Title:         "Sub Feature",
			Status:        "pending",
			ParentID:      "01",
			Depth:         1,
			ContextBudget: 5000,
		},
	}

	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m.SetPath(manifestPath)

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.MaxDepth != 5 {
		t.Errorf("expected max depth 5, got %d", loaded.MaxDepth)
	}

	root := loaded.GetFeature("01")
	if root == nil {
		t.Fatal("root not found")
	}
	if len(root.Children) != 1 || root.Children[0] != "01-01" {
		t.Errorf("root children mismatch: %v", root.Children)
	}
	if root.ContextBudget != 10000 {
		t.Errorf("expected context budget 10000, got %d", root.ContextBudget)
	}

	child := loaded.GetFeature("01-01")
	if child == nil {
		t.Fatal("child not found")
	}
	if child.ParentID != "01" {
		t.Errorf("expected parent ID '01', got %q", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected depth 1, got %d", child.Depth)
	}
	if child.ContextBudget != 5000 {
		t.Errorf("expected context budget 5000, got %d", child.ContextBudget)
	}
}

func TestBackwardCompatibilityFlatManifest(t *testing.T) {
	flatJSON := `{
		"source": "test.md",
		"title": "Test Project",
		"created": "2024-01-01T00:00:00Z",
		"updated": "2024-01-01T00:00:00Z",
		"features": [
			{
				"id": "01",
				"dir": "01-feature-one",
				"title": "Feature One",
				"status": "completed",
				"depends_on": [],
				"execution": "sequential",
				"model": "sonnet"
			},
			{
				"id": "02",
				"dir": "02-feature-two",
				"title": "Feature Two",
				"status": "pending",
				"depends_on": ["01"],
				"execution": "parallel",
				"model": "opus"
			}
		]
	}`

	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(flatJSON), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	m, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load flat manifest: %v", err)
	}

	if len(m.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(m.Features))
	}

	if m.GetMaxDepth() != DefaultMaxDepth {
		t.Errorf("expected default max depth for flat manifest, got %d", m.GetMaxDepth())
	}

	f1 := m.GetFeature("01")
	if f1 == nil {
		t.Fatal("feature 01 not found")
	}
	if !f1.IsRootFeature() {
		t.Error("feature 01 should be a root feature (empty parent ID)")
	}
	if f1.Depth != 0 {
		t.Errorf("expected depth 0 for flat feature, got %d", f1.Depth)
	}
	if f1.HasChildren() {
		t.Error("flat features should have no children")
	}

	roots := m.GetRootFeatures()
	if len(roots) != 2 {
		t.Errorf("all flat features should be roots, got %d", len(roots))
	}
}

func TestContextBudgetInheritance(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Depth: 0, ContextBudget: 100000},
	}

	child1 := ManifestFeature{ID: "01-01", Title: "Child 1"}
	if err := m.AddSubFeature("01", child1); err != nil {
		t.Fatalf("failed to add child 1: %v", err)
	}

	c1 := m.GetFeature("01-01")
	if c1.ContextBudget != 50000 {
		t.Errorf("child 1 should have 50000 (half of 100000), got %d", c1.ContextBudget)
	}

	grandchild := ManifestFeature{ID: "01-01-01", Title: "Grandchild"}
	if err := m.AddSubFeature("01-01", grandchild); err != nil {
		t.Fatalf("failed to add grandchild: %v", err)
	}

	gc := m.GetFeature("01-01-01")
	if gc.ContextBudget != 25000 {
		t.Errorf("grandchild should have 25000 (half of 50000), got %d", gc.ContextBudget)
	}

	child2 := ManifestFeature{ID: "01-02", Title: "Child 2 with custom budget", ContextBudget: 80000}
	if err := m.AddSubFeature("01", child2); err != nil {
		t.Fatalf("failed to add child 2: %v", err)
	}

	c2 := m.GetFeature("01-02")
	if c2.ContextBudget != 80000 {
		t.Errorf("child 2 should keep explicit budget 80000, got %d", c2.ContextBudget)
	}
}

func TestDepthTracking(t *testing.T) {
	m := New("test.md", "Test Project")
	m.Features = []ManifestFeature{
		{ID: "01", Title: "Root", Depth: 0},
	}

	for i := 1; i <= 3; i++ {
		parentID := "01"
		if i > 1 {
			parentID = m.Features[len(m.Features)-1].ID
		}
		child := ManifestFeature{
			ID:    "01" + strings.Repeat("-01", i),
			Title: "Level " + strconv.Itoa(i),
		}
		if err := m.AddSubFeature(parentID, child); err != nil {
			t.Fatalf("failed to add level %d: %v", i, err)
		}
	}

	for i, f := range m.Features {
		if f.Depth != i {
			t.Errorf("feature %q expected depth %d, got %d", f.ID, i, f.Depth)
		}
	}
}
