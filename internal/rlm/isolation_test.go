package rlm

import (
	"testing"
	"time"
)

func TestIsolationLevelConstants(t *testing.T) {
	if IsolationStrict != "strict" {
		t.Errorf("IsolationStrict = %q, want %q", IsolationStrict, "strict")
	}
	if IsolationLenient != "lenient" {
		t.Errorf("IsolationLenient = %q, want %q", IsolationLenient, "lenient")
	}
	if DefaultIsolationLevel != IsolationLenient {
		t.Errorf("DefaultIsolationLevel = %q, want %q", DefaultIsolationLevel, IsolationLenient)
	}
}

func TestNewFailureInfo(t *testing.T) {
	reason := "test_failure"
	errMsg := "something went wrong"

	info := NewFailureInfo(reason, errMsg)

	if info == nil {
		t.Fatal("NewFailureInfo returned nil")
	}
	if info.Reason != reason {
		t.Errorf("Reason = %q, want %q", info.Reason, reason)
	}
	if info.Error != errMsg {
		t.Errorf("Error = %q, want %q", info.Error, errMsg)
	}
	if !info.Recoverable {
		t.Error("Expected Recoverable to be true")
	}
	if info.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", info.RetryCount)
	}
	if info.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", info.MaxRetries)
	}
	if info.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestFailureInfoCanRetry(t *testing.T) {
	tests := []struct {
		name        string
		recoverable bool
		retryCount  int
		maxRetries  int
		want        bool
	}{
		{
			name:        "can retry - fresh",
			recoverable: true,
			retryCount:  0,
			maxRetries:  3,
			want:        true,
		},
		{
			name:        "can retry - some retries left",
			recoverable: true,
			retryCount:  2,
			maxRetries:  3,
			want:        true,
		},
		{
			name:        "cannot retry - max reached",
			recoverable: true,
			retryCount:  3,
			maxRetries:  3,
			want:        false,
		},
		{
			name:        "cannot retry - not recoverable",
			recoverable: false,
			retryCount:  0,
			maxRetries:  3,
			want:        false,
		},
		{
			name:        "cannot retry - exceeded",
			recoverable: true,
			retryCount:  5,
			maxRetries:  3,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &FailureInfo{
				Recoverable: tt.recoverable,
				RetryCount:  tt.retryCount,
				MaxRetries:  tt.maxRetries,
			}
			if got := info.CanRetry(); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFailureInfoIncrementRetry(t *testing.T) {
	info := NewFailureInfo("test", "error")

	if info.RetryCount != 0 {
		t.Fatalf("Initial RetryCount = %d, want 0", info.RetryCount)
	}

	info.IncrementRetry()
	if info.RetryCount != 1 {
		t.Errorf("After first increment, RetryCount = %d, want 1", info.RetryCount)
	}

	info.IncrementRetry()
	if info.RetryCount != 2 {
		t.Errorf("After second increment, RetryCount = %d, want 2", info.RetryCount)
	}
}

func TestChildFailureActionConstants(t *testing.T) {
	if ChildFailureRetry != "retry" {
		t.Errorf("ChildFailureRetry = %q, want %q", ChildFailureRetry, "retry")
	}
	if ChildFailureSkip != "skip" {
		t.Errorf("ChildFailureSkip = %q, want %q", ChildFailureSkip, "skip")
	}
	if ChildFailureAbort != "abort" {
		t.Errorf("ChildFailureAbort = %q, want %q", ChildFailureAbort, "abort")
	}
	if ChildFailureHandle != "handle" {
		t.Errorf("ChildFailureHandle = %q, want %q", ChildFailureHandle, "handle")
	}
}

func TestChildFailureResult(t *testing.T) {
	result := &ChildFailureResult{
		ChildID:    "child-123",
		ChildTitle: "Test Child",
		ParentID:   "parent-456",
		FailureInfo: &FailureInfo{
			Reason: "test_failure",
			Error:  "something broke",
		},
		Action:     ChildFailureHandle,
		SkipReason: "skipped for testing",
	}

	if result.ChildID != "child-123" {
		t.Errorf("ChildID = %q, want %q", result.ChildID, "child-123")
	}
	if result.ChildTitle != "Test Child" {
		t.Errorf("ChildTitle = %q, want %q", result.ChildTitle, "Test Child")
	}
	if result.ParentID != "parent-456" {
		t.Errorf("ParentID = %q, want %q", result.ParentID, "parent-456")
	}
	if result.Action != ChildFailureHandle {
		t.Errorf("Action = %q, want %q", result.Action, ChildFailureHandle)
	}
}

func TestRecursiveFeatureGetIsolationLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    IsolationLevel
		expected IsolationLevel
	}{
		{
			name:     "empty returns default",
			level:    "",
			expected: DefaultIsolationLevel,
		},
		{
			name:     "strict returns strict",
			level:    IsolationStrict,
			expected: IsolationStrict,
		},
		{
			name:     "lenient returns lenient",
			level:    IsolationLenient,
			expected: IsolationLenient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &RecursiveFeature{
				IsolationLevel: tt.level,
			}
			if got := f.GetIsolationLevel(); got != tt.expected {
				t.Errorf("GetIsolationLevel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRecursiveFeatureSetIsolationLevel(t *testing.T) {
	f := NewRecursiveFeature("test-1", "Test Feature")

	f.SetIsolationLevel(IsolationStrict)
	if f.IsolationLevel != IsolationStrict {
		t.Errorf("IsolationLevel = %q, want %q", f.IsolationLevel, IsolationStrict)
	}

	f.SetIsolationLevel(IsolationLenient)
	if f.IsolationLevel != IsolationLenient {
		t.Errorf("IsolationLevel = %q, want %q", f.IsolationLevel, IsolationLenient)
	}
}

func TestRecursiveFeatureGetSetFailureInfo(t *testing.T) {
	f := NewRecursiveFeature("test-1", "Test Feature")

	// Initially nil
	if f.GetFailureInfo() != nil {
		t.Error("Expected initial FailureInfo to be nil")
	}

	// Set failure info
	info := NewFailureInfo("test_reason", "test error")
	f.SetFailureInfo(info)

	got := f.GetFailureInfo()
	if got == nil {
		t.Fatal("Expected FailureInfo to be set")
	}
	if got.Reason != "test_reason" {
		t.Errorf("Reason = %q, want %q", got.Reason, "test_reason")
	}
}

func TestRecursiveFeatureRecordFailure(t *testing.T) {
	f := NewRecursiveFeature("test-1", "Test Feature")
	f.SetStatus("running")

	f.RecordFailure("execution_error", "test failure message")

	if f.Status != "failed" {
		t.Errorf("Status = %q, want %q", f.Status, "failed")
	}
	if f.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	info := f.GetFailureInfo()
	if info == nil {
		t.Fatal("Expected FailureInfo to be set")
	}
	if info.Reason != "execution_error" {
		t.Errorf("Reason = %q, want %q", info.Reason, "execution_error")
	}
	if info.Error != "test failure message" {
		t.Errorf("Error = %q, want %q", info.Error, "test failure message")
	}
}

func TestRecursiveFeatureAddFailedChild(t *testing.T) {
	f := NewRecursiveFeature("parent-1", "Parent Feature")

	// Initially empty
	if len(f.GetFailedChildren()) != 0 {
		t.Error("Expected no failed children initially")
	}

	// Add first child
	f.AddFailedChild("child-1")
	children := f.GetFailedChildren()
	if len(children) != 1 {
		t.Fatalf("Expected 1 failed child, got %d", len(children))
	}
	if children[0] != "child-1" {
		t.Errorf("Failed child = %q, want %q", children[0], "child-1")
	}

	// Add second child
	f.AddFailedChild("child-2")
	children = f.GetFailedChildren()
	if len(children) != 2 {
		t.Fatalf("Expected 2 failed children, got %d", len(children))
	}

	// Add duplicate - should not add again
	f.AddFailedChild("child-1")
	children = f.GetFailedChildren()
	if len(children) != 2 {
		t.Errorf("Expected 2 failed children after duplicate, got %d", len(children))
	}
}

func TestRecursiveFeatureHasFailedChildren(t *testing.T) {
	f := NewRecursiveFeature("parent-1", "Parent Feature")

	if f.HasFailedChildren() {
		t.Error("Expected HasFailedChildren() to be false initially")
	}

	f.AddFailedChild("child-1")
	if !f.HasFailedChildren() {
		t.Error("Expected HasFailedChildren() to be true after adding failed child")
	}
}

func TestRecursiveFeatureShouldFailOnChildFailure(t *testing.T) {
	tests := []struct {
		name     string
		level    IsolationLevel
		expected bool
	}{
		{
			name:     "strict should fail on child failure",
			level:    IsolationStrict,
			expected: true,
		},
		{
			name:     "lenient should not fail on child failure",
			level:    IsolationLenient,
			expected: false,
		},
		{
			name:     "empty (default) should not fail on child failure",
			level:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &RecursiveFeature{
				IsolationLevel: tt.level,
			}
			if got := f.ShouldFailOnChildFailure(); got != tt.expected {
				t.Errorf("ShouldFailOnChildFailure() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRecursiveFeatureClearFailure(t *testing.T) {
	f := NewRecursiveFeature("test-1", "Test Feature")

	// Set up failure state
	f.RecordFailure("test_reason", "test error")
	f.AddFailedChild("child-1")
	f.AddFailedChild("child-2")

	// Verify state is set
	if f.GetFailureInfo() == nil {
		t.Error("Expected FailureInfo to be set before clear")
	}
	if len(f.GetFailedChildren()) == 0 {
		t.Error("Expected FailedChildren to be set before clear")
	}

	// Clear failure
	f.ClearFailure()

	if f.GetFailureInfo() != nil {
		t.Error("Expected FailureInfo to be nil after clear")
	}
	if len(f.GetFailedChildren()) != 0 {
		t.Error("Expected FailedChildren to be empty after clear")
	}
}

func TestIsolationConcurrency(t *testing.T) {
	f := NewRecursiveFeature("test-1", "Test Feature")

	done := make(chan bool)

	// Concurrent readers and writers
	for i := 0; i < 10; i++ {
		go func() {
			f.SetIsolationLevel(IsolationStrict)
			_ = f.GetIsolationLevel()
			done <- true
		}()
		go func() {
			f.SetIsolationLevel(IsolationLenient)
			_ = f.GetIsolationLevel()
			done <- true
		}()
		go func() {
			f.AddFailedChild("child-" + time.Now().String())
			_ = f.GetFailedChildren()
			_ = f.HasFailedChildren()
			done <- true
		}()
	}

	for i := 0; i < 30; i++ {
		<-done
	}
}
