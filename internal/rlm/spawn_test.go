package rlm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewSpawnHandler(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.manager != mgr {
		t.Error("handler manager not set correctly")
	}
}

func TestSpawnHandlerRegisterRootFeature(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	feature := handler.RegisterRootFeature("01", "Test Feature")

	if feature == nil {
		t.Fatal("expected non-nil feature")
	}
	if feature.ID != "01" {
		t.Errorf("expected ID '01', got %q", feature.ID)
	}
	if feature.Title != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got %q", feature.Title)
	}
	if feature.Depth != 0 {
		t.Errorf("expected depth 0, got %d", feature.Depth)
	}
	if !feature.IsRoot() {
		t.Error("expected feature to be root")
	}
}

func TestSpawnHandlerSetFeatureRunning(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	handler.RegisterRootFeature("01", "Test")
	handler.SetFeatureRunning("01")

	feature := handler.GetFeature("01")
	if feature.GetStatus() != "running" {
		t.Errorf("expected status 'running', got %q", feature.GetStatus())
	}
}

func TestSpawnHandlerProcessLine(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	feature := handler.RegisterRootFeature("01", "Test")
	feature.SetStatus("running")

	spawnJSON := `{
		"type": "tool_use",
		"tool": "ralph_spawn_feature",
		"tool_input": {"title": "Child Feature", "tasks": ["Task 1", "Task 2"]}
	}`

	req, err := handler.ProcessLine("01", spawnJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected spawn request")
	}
	if req.Title != "Child Feature" {
		t.Errorf("expected title 'Child Feature', got %q", req.Title)
	}
	if len(req.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(req.Tasks))
	}
}

func TestSpawnHandlerProcessLineNoSpawn(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	handler.RegisterRootFeature("01", "Test")

	normalJSON := `{"type": "assistant", "content": "Hello"}`

	req, err := handler.ProcessLine("01", normalJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req != nil {
		t.Error("expected nil spawn request for normal message")
	}
}

func TestSpawnHandlerProcessLineUnregisteredFeature(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	req, err := handler.ProcessLine("nonexistent", `{"type":"assistant"}`)
	if err != nil {
		t.Error("expected nil error for unregistered feature")
	}
	if req != nil {
		t.Error("expected nil request for unregistered feature")
	}
}

func TestSpawnHandlerSpawnChild(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	parent := handler.RegisterRootFeature("01", "Parent")
	parent.SetStatus("running")

	req := &SpawnRequest{
		Title:       "Child Feature",
		Tasks:       []string{"Task 1", "Task 2"},
		Model:       "haiku",
		Description: "A child feature",
	}

	child, err := handler.SpawnChild("01", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
	if child.Title != "Child Feature" {
		t.Errorf("expected title 'Child Feature', got %q", child.Title)
	}
	if child.ParentID != "01" {
		t.Errorf("expected parent ID '01', got %q", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected depth 1, got %d", child.Depth)
	}
	if child.Model != "haiku" {
		t.Errorf("expected model 'haiku', got %q", child.Model)
	}
	if len(child.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(child.Tasks))
	}

	subs := parent.GetSubFeatures()
	if len(subs) != 1 {
		t.Errorf("expected 1 sub-feature, got %d", len(subs))
	}
}

func TestSpawnHandlerSpawnChildDepthLimit(t *testing.T) {
	mgr := NewManagerWithConfig(2, 100000)
	handler := NewSpawnHandler(mgr, nil)

	parent := handler.RegisterRootFeature("01", "Parent")
	parent.SetStatus("running")

	child1, err := handler.SpawnChild("01", &SpawnRequest{Title: "Child 1"})
	if err != nil {
		t.Fatalf("unexpected error spawning child 1: %v", err)
	}
	child1.SetStatus("running")

	child2, err := handler.SpawnChild(child1.ID, &SpawnRequest{Title: "Child 2"})
	if err != nil {
		t.Fatalf("unexpected error spawning child 2: %v", err)
	}
	child2.SetStatus("running")

	_, err = handler.SpawnChild(child2.ID, &SpawnRequest{Title: "Child 3"})
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestSpawnHandlerSpawnChildParentNotRunning(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	handler.RegisterRootFeature("01", "Parent")

	_, err := handler.SpawnChild("01", &SpawnRequest{Title: "Child"})
	if err != ErrParentNotRunning {
		t.Errorf("expected ErrParentNotRunning, got %v", err)
	}
}

func TestSpawnHandlerCompleteFeature(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	parent := handler.RegisterRootFeature("01", "Parent")
	parent.SetStatus("running")

	child, _ := handler.SpawnChild("01", &SpawnRequest{Title: "Child"})
	child.SetStatus("running")
	child.TokenUsage.Update(1000, 500, 0, 0, 0.01)

	context := handler.CompleteFeature(child.ID, "completed", "Child completed successfully")

	if child.GetStatus() != "completed" {
		t.Errorf("expected status 'completed', got %q", child.GetStatus())
	}

	if context == "" {
		t.Error("expected non-empty context")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(context), &data); err != nil {
		t.Fatalf("context is not valid JSON: %v", err)
	}

	sub, ok := data["sub_feature_completed"].(map[string]interface{})
	if !ok {
		t.Fatal("missing sub_feature_completed in context")
	}
	if sub["status"] != "completed" {
		t.Errorf("expected status 'completed' in context, got %v", sub["status"])
	}
}

func TestSpawnHandlerCanSpawn(t *testing.T) {
	mgr := NewManagerWithConfig(2, 100000)
	handler := NewSpawnHandler(mgr, nil)

	parent := handler.RegisterRootFeature("01", "Parent")

	if !handler.CanSpawn("01") {
		t.Error("root feature should be able to spawn")
	}

	parent.SetStatus("running")
	child1, _ := handler.SpawnChild("01", &SpawnRequest{Title: "Child 1"})

	if !handler.CanSpawn(child1.ID) {
		t.Error("depth 1 should be able to spawn")
	}

	child1.SetStatus("running")
	child2, _ := handler.SpawnChild(child1.ID, &SpawnRequest{Title: "Child 2"})

	if handler.CanSpawn(child2.ID) {
		t.Error("depth 2 (at max) should not be able to spawn")
	}
}

func TestSpawnHandlerGetDepth(t *testing.T) {
	mgr := NewManager()
	handler := NewSpawnHandler(mgr, nil)

	parent := handler.RegisterRootFeature("01", "Parent")
	parent.SetStatus("running")

	if handler.GetDepth("01") != 0 {
		t.Errorf("expected depth 0 for root, got %d", handler.GetDepth("01"))
	}

	child, _ := handler.SpawnChild("01", &SpawnRequest{Title: "Child"})
	if handler.GetDepth(child.ID) != 1 {
		t.Errorf("expected depth 1 for child, got %d", handler.GetDepth(child.ID))
	}

	if handler.GetDepth("nonexistent") != 0 {
		t.Error("expected depth 0 for nonexistent feature")
	}
}

func TestSpawnHandlerBuildChildPrompt(t *testing.T) {
	handler := NewSpawnHandler(NewManager(), nil)

	req := &SpawnRequest{
		Title:       "Test Child",
		Description: "This is a test child feature",
		Tasks:       []string{"Task 1", "Task 2"},
	}

	prompt := handler.BuildChildPrompt(req, "Parent context here")

	if !strings.Contains(prompt, "# Sub-Feature: Test Child") {
		t.Error("prompt missing title")
	}
	if !strings.Contains(prompt, "This is a test child feature") {
		t.Error("prompt missing description")
	}
	if !strings.Contains(prompt, "- [ ] Task 1") {
		t.Error("prompt missing task 1")
	}
	if !strings.Contains(prompt, "- [ ] Task 2") {
		t.Error("prompt missing task 2")
	}
	if !strings.Contains(prompt, "Parent context here") {
		t.Error("prompt missing parent context")
	}
}

func TestSpawnHandlerBuildChildPromptMinimal(t *testing.T) {
	handler := NewSpawnHandler(NewManager(), nil)

	req := &SpawnRequest{
		Title: "Minimal Child",
	}

	prompt := handler.BuildChildPrompt(req, "")

	if !strings.Contains(prompt, "# Sub-Feature: Minimal Child") {
		t.Error("prompt missing title")
	}
	if strings.Contains(prompt, "## Description") {
		t.Error("prompt should not have description section")
	}
	if strings.Contains(prompt, "## Context from Parent") {
		t.Error("prompt should not have parent context section")
	}
}

func TestSpawnHandlerNilManager(t *testing.T) {
	handler := NewSpawnHandler(nil, nil)

	if handler.RegisterRootFeature("01", "Test") != nil {
		t.Error("expected nil from nil manager")
	}
	if handler.GetFeature("01") != nil {
		t.Error("expected nil from nil manager")
	}
	if handler.CanSpawn("01") {
		t.Error("expected false from nil manager")
	}
	if handler.GetDepth("01") != 0 {
		t.Error("expected 0 from nil manager")
	}
	if handler.GetManager() != nil {
		t.Error("expected nil manager")
	}

	req, err := handler.ProcessLine("01", `{}`)
	if req != nil || err != nil {
		t.Error("expected nil req and nil err from nil manager")
	}

	_, err = handler.SpawnChild("01", &SpawnRequest{Title: "Test"})
	if err != ErrInvalidSpawnData {
		t.Errorf("expected ErrInvalidSpawnData from nil manager, got %v", err)
	}
}
