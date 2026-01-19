package auto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vx/ralph-go/internal/manifest"
)

func TestPRDDirExists(t *testing.T) {
	t.Run("returns false when PRD/ not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		if PRDDirExists() {
			t.Error("expected false when PRD/ not found")
		}
	})

	t.Run("returns true when PRD/ exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		os.Mkdir(filepath.Join(tmpDir, "PRD"), 0755)

		if !PRDDirExists() {
			t.Error("expected true when PRD/ exists")
		}
	})
}

func TestFindPRDDir(t *testing.T) {
	t.Run("returns error when PRD/ not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		_, err := FindPRDDir()
		if err == nil {
			t.Error("expected error when PRD/ not found")
		}
	})

	t.Run("returns error when manifest.json not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		os.Mkdir(filepath.Join(tmpDir, "PRD"), 0755)

		_, err := FindPRDDir()
		if err == nil {
			t.Error("expected error when manifest.json not found")
		}
	})

	t.Run("returns PRD path when valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer os.Chdir(origDir)
		os.Chdir(tmpDir)

		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)
		os.WriteFile(filepath.Join(prdDir, "manifest.json"), []byte("{}"), 0644)

		path, err := FindPRDDir()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != prdDir {
			t.Errorf("expected %s, got %s", prdDir, path)
		}
	})
}

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "PRD")
	os.Mkdir(prdDir, 0755)

	m := &manifest.Manifest{
		Source:  "test.md",
		Title:   "Test Project",
		Created: time.Now(),
		Updated: time.Now(),
		Features: []manifest.ManifestFeature{
			{ID: "01", Dir: "01-feature-one", Title: "Feature One", Status: "pending"},
		},
	}

	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(prdDir, "manifest.json"), data, 0644)

	loaded, err := LoadManifest(prdDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if loaded.Title != "Test Project" {
		t.Errorf("expected title 'Test Project', got '%s'", loaded.Title)
	}
	if len(loaded.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(loaded.Features))
	}
}

func TestGetFeaturePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	prdDir := filepath.Join(tmpDir, "PRD")
	featureDir := filepath.Join(prdDir, "01-test-feature")
	os.MkdirAll(featureDir, 0755)

	content := "# Test Feature\n\nThis is a test feature."
	os.WriteFile(filepath.Join(featureDir, "feature.md"), []byte(content), 0644)

	feature := &manifest.ManifestFeature{
		ID:    "01",
		Dir:   "01-test-feature",
		Title: "Test Feature",
	}

	prompt, err := GetFeaturePrompt(prdDir, feature)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if prompt != content {
		t.Errorf("expected '%s', got '%s'", content, prompt)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		expected int
	}{
		{
			name:     "completed returns 0",
			result:   &Result{Status: "completed"},
			expected: 0,
		},
		{
			name:     "failed returns 1",
			result:   &Result{Status: "failed"},
			expected: 1,
		},
		{
			name:     "no work returns 0",
			result:   &Result{NoWork: true},
			expected: 0,
		},
		{
			name:     "all_completed returns 0",
			result:   &Result{NoWork: true, Status: "all_completed"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := ExitCode(tt.result)
			if code != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, code)
			}
		})
	}
}

func TestHandleNoRunnableFeature(t *testing.T) {
	t.Run("all completed", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		m := manifest.New("test.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
			{ID: "02", Title: "Feature 2", Status: "completed"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		result, err := handleNoRunnableFeature(m)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.NoWork {
			t.Error("expected NoWork to be true")
		}
		if result.Status != "all_completed" {
			t.Errorf("expected status 'all_completed', got '%s'", result.Status)
		}
	})

	t.Run("blocked by dependencies", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		m := manifest.New("test.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "failed"},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		result, err := handleNoRunnableFeature(m)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.NoWork {
			t.Error("expected NoWork to be true")
		}
		if result.Status != "blocked_by_failures" {
			t.Errorf("expected status 'blocked_by_failures', got '%s'", result.Status)
		}
		if len(result.Blocked) != 1 {
			t.Errorf("expected 1 blocked feature, got %d", len(result.Blocked))
		}
	})

	t.Run("running elsewhere", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		m := manifest.New("test.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "running"},
			{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		result, err := handleNoRunnableFeature(m)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Status != "running_elsewhere" {
			t.Errorf("expected status 'running_elsewhere', got '%s'", result.Status)
		}
	})
}

func TestGetBlockedFeatures(t *testing.T) {
	m := manifest.New("test.md", "Test")
	m.Features = []manifest.ManifestFeature{
		{ID: "01", Title: "Feature 1", Status: "failed"},
		{ID: "02", Title: "Feature 2", Status: "pending", DependsOn: []string{"01"}},
		{ID: "03", Title: "Feature 3", Status: "pending", DependsOn: []string{"01", "02"}},
	}

	blocked := getBlockedFeatures(m)

	if len(blocked) != 2 {
		t.Errorf("expected 2 blocked features, got %d", len(blocked))
	}

	found02 := false
	found03 := false
	for _, bf := range blocked {
		if bf.ID == "02" {
			found02 = true
			if len(bf.PendingDeps) != 1 || bf.PendingDeps[0] != "01" {
				t.Errorf("feature 02 should have pending dep 01")
			}
		}
		if bf.ID == "03" {
			found03 = true
			if len(bf.PendingDeps) != 2 {
				t.Errorf("feature 03 should have 2 pending deps")
			}
		}
	}

	if !found02 || !found03 {
		t.Error("expected to find features 02 and 03 in blocked list")
	}
}

func TestCheckAndArchivePRD(t *testing.T) {
	t.Run("archives PRD when all features completed", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		prdContent := "# Test PRD\n\n## Feature 1\n\nSome content"
		prdPath := filepath.Join(tmpDir, "test.md")
		os.WriteFile(prdPath, []byte(prdContent), 0644)

		m := manifest.New("test.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
			{ID: "02", Title: "Feature 2", Status: "completed"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		archived, archivePath := checkAndArchivePRD(prdDir, m)

		if !archived {
			t.Error("expected PRD to be archived")
		}

		expectedArchivePath := filepath.Join(prdDir, "test_authored.md")
		if archivePath != expectedArchivePath {
			t.Errorf("expected archive path %s, got %s", expectedArchivePath, archivePath)
		}

		if _, err := os.Stat(prdPath); !os.IsNotExist(err) {
			t.Error("original PRD should have been moved")
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Error("archived PRD should exist")
		}

		content, _ := os.ReadFile(archivePath)
		if string(content) != prdContent {
			t.Error("archived content should match original")
		}

		reloadedManifest, _ := manifest.Load(prdDir)
		if reloadedManifest.Source != "test_authored.md" {
			t.Errorf("manifest source should be updated, got %s", reloadedManifest.Source)
		}
	})

	t.Run("does not archive when not all features completed", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		prdPath := filepath.Join(tmpDir, "test.md")
		os.WriteFile(prdPath, []byte("# Test"), 0644)

		m := manifest.New("test.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
			{ID: "02", Title: "Feature 2", Status: "pending"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		archived, _ := checkAndArchivePRD(prdDir, m)

		if archived {
			t.Error("should not archive when features are pending")
		}

		if _, err := os.Stat(prdPath); os.IsNotExist(err) {
			t.Error("original PRD should still exist")
		}
	})

	t.Run("idempotent when PRD already archived", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		archivedPath := filepath.Join(prdDir, "test_authored.md")
		os.WriteFile(archivedPath, []byte("# Test"), 0644)

		m := manifest.New("test_authored.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		archived, _ := checkAndArchivePRD(prdDir, m)

		if archived {
			t.Error("should not archive again when already archived")
		}
	})

	t.Run("handles missing source PRD gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		m := manifest.New("nonexistent.md", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		archived, _ := checkAndArchivePRD(prdDir, m)

		if archived {
			t.Error("should not archive when source does not exist")
		}
	})

	t.Run("handles empty source gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdDir := filepath.Join(tmpDir, "PRD")
		os.Mkdir(prdDir, 0755)

		m := manifest.New("", "Test")
		m.Features = []manifest.ManifestFeature{
			{ID: "01", Title: "Feature 1", Status: "completed"},
		}
		m.SetPath(filepath.Join(prdDir, "manifest.json"))
		m.Save()

		archived, _ := checkAndArchivePRD(prdDir, m)

		if archived {
			t.Error("should not archive when source is empty")
		}
	})
}
