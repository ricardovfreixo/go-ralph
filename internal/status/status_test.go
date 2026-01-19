package status

import (
	"testing"

	"github.com/vx/ralph-go/internal/manifest"
)

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		depsSatisfied bool
		wantIcon      string
		wantColor     string
	}{
		{
			name:          "completed",
			status:        "completed",
			depsSatisfied: true,
			wantIcon:      iconCompleted,
			wantColor:     colorGreen,
		},
		{
			name:          "running",
			status:        "running",
			depsSatisfied: true,
			wantIcon:      iconRunning,
			wantColor:     colorYellow,
		},
		{
			name:          "failed",
			status:        "failed",
			depsSatisfied: true,
			wantIcon:      iconFailed,
			wantColor:     colorRed,
		},
		{
			name:          "pending with deps satisfied",
			status:        "pending",
			depsSatisfied: true,
			wantIcon:      iconPending,
			wantColor:     colorGray,
		},
		{
			name:          "pending with deps not satisfied (blocked)",
			status:        "pending",
			depsSatisfied: false,
			wantIcon:      iconBlocked,
			wantColor:     colorGray,
		},
		{
			name:          "unknown status",
			status:        "unknown",
			depsSatisfied: true,
			wantIcon:      iconPending,
			wantColor:     colorGray,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIcon, gotColor := getStatusIcon(tt.status, tt.depsSatisfied)
			if gotIcon != tt.wantIcon {
				t.Errorf("getStatusIcon() icon = %q, want %q", gotIcon, tt.wantIcon)
			}
			if gotColor != tt.wantColor {
				t.Errorf("getStatusIcon() color = %q, want %q", gotColor, tt.wantColor)
			}
		})
	}
}

func TestFormatDeps(t *testing.T) {
	tests := []struct {
		name string
		deps []string
		want string
	}{
		{
			name: "empty deps",
			deps: []string{},
			want: "",
		},
		{
			name: "nil deps",
			deps: nil,
			want: "",
		},
		{
			name: "single dep",
			deps: []string{"01"},
			want: "→01",
		},
		{
			name: "multiple deps",
			deps: []string{"01", "02"},
			want: "→01,02",
		},
		{
			name: "three deps",
			deps: []string{"01", "02", "03"},
			want: "→01,02,03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDeps(tt.deps)
			if got != tt.want {
				t.Errorf("formatDeps() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPendingDepTitles(t *testing.T) {
	m := manifest.New("test.md", "Test PRD")
	m.Features = append(m.Features, manifest.ManifestFeature{
		ID:     "01",
		Title:  "Feature One",
		Status: "completed",
	})
	m.Features = append(m.Features, manifest.ManifestFeature{
		ID:     "02",
		Title:  "Feature Two",
		Status: "pending",
	})
	m.Features = append(m.Features, manifest.ManifestFeature{
		ID:     "03",
		Title:  "Feature Three",
		Status: "failed",
	})

	tests := []struct {
		name       string
		pendingIDs []string
		want       []string
	}{
		{
			name:       "single pending",
			pendingIDs: []string{"02"},
			want:       []string{"Feature Two (pending)"},
		},
		{
			name:       "multiple pending",
			pendingIDs: []string{"02", "03"},
			want:       []string{"Feature Two (pending)", "Feature Three (failed)"},
		},
		{
			name:       "unknown feature",
			pendingIDs: []string{"99"},
			want:       []string{"99"},
		},
		{
			name:       "mixed known and unknown",
			pendingIDs: []string{"02", "99"},
			want:       []string{"Feature Two (pending)", "99"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPendingDepTitles(m, tt.pendingIDs)
			if len(got) != len(tt.want) {
				t.Errorf("getPendingDepTitles() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("getPendingDepTitles()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestIconsAreDefined(t *testing.T) {
	if iconCompleted == "" {
		t.Error("iconCompleted should not be empty")
	}
	if iconRunning == "" {
		t.Error("iconRunning should not be empty")
	}
	if iconFailed == "" {
		t.Error("iconFailed should not be empty")
	}
	if iconPending == "" {
		t.Error("iconPending should not be empty")
	}
	if iconBlocked == "" {
		t.Error("iconBlocked should not be empty")
	}
}

func TestColorsAreDefined(t *testing.T) {
	if colorReset == "" {
		t.Error("colorReset should not be empty")
	}
	if colorGreen == "" {
		t.Error("colorGreen should not be empty")
	}
	if colorYellow == "" {
		t.Error("colorYellow should not be empty")
	}
	if colorRed == "" {
		t.Error("colorRed should not be empty")
	}
	if colorGray == "" {
		t.Error("colorGray should not be empty")
	}
}
