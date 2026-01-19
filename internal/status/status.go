package status

import (
	"fmt"
	"strings"

	"github.com/vx/ralph-go/internal/auto"
	"github.com/vx/ralph-go/internal/manifest"
)

const (
	iconCompleted = "✓"
	iconRunning   = "●"
	iconFailed    = "✗"
	iconPending   = "○"
	iconBlocked   = "◌"

	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func Run() error {
	prdDir, err := auto.FindPRDDir()
	if err != nil {
		return err
	}

	m, err := auto.LoadManifest(prdDir)
	if err != nil {
		return err
	}

	printStatus(m)
	return nil
}

func printStatus(m *manifest.Manifest) {
	total, completed, running, failed, pending, blocked := m.GetSummary()

	fmt.Println()
	fmt.Printf("%s%s%s\n", colorBold, m.Title, colorReset)
	fmt.Println(strings.Repeat("─", len(m.Title)))
	fmt.Println()

	features := m.AllFeatures()
	for _, f := range features {
		printFeature(m, &f)
	}

	fmt.Println()
	printSummary(total, completed, running, failed, pending, blocked)
	fmt.Println()
}

func printFeature(m *manifest.Manifest, f *manifest.ManifestFeature) {
	icon, color := getStatusIcon(f.Status, m.IsDependencySatisfied(f.ID))

	deps := formatDeps(f.DependsOn)
	depsStr := ""
	if deps != "" {
		depsStr = fmt.Sprintf(" %s[%s]%s", colorDim, deps, colorReset)
	}

	fmt.Printf("  %s%s%s %s %s%s\n", color, icon, colorReset, f.ID, f.Title, depsStr)

	if f.Status == "pending" && !m.IsDependencySatisfied(f.ID) {
		pending := m.GetPendingDependencies(f.ID)
		if len(pending) > 0 {
			pendingTitles := getPendingDepTitles(m, pending)
			fmt.Printf("      %s↳ waiting on: %s%s\n", colorGray, strings.Join(pendingTitles, ", "), colorReset)
		}
	}
}

func getStatusIcon(status string, depsSatisfied bool) (string, string) {
	switch status {
	case "completed":
		return iconCompleted, colorGreen
	case "running":
		return iconRunning, colorYellow
	case "failed":
		return iconFailed, colorRed
	case "pending":
		if depsSatisfied {
			return iconPending, colorGray
		}
		return iconBlocked, colorGray
	default:
		return iconPending, colorGray
	}
}

func formatDeps(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	return "→" + strings.Join(deps, ",")
}

func getPendingDepTitles(m *manifest.Manifest, pendingIDs []string) []string {
	titles := make([]string, 0, len(pendingIDs))
	for _, id := range pendingIDs {
		if f := m.GetFeature(id); f != nil {
			titles = append(titles, fmt.Sprintf("%s (%s)", f.Title, f.Status))
		} else {
			titles = append(titles, id)
		}
	}
	return titles
}

func printSummary(total, completed, running, failed, pending, blocked int) {
	fmt.Printf("Summary: ")

	parts := []string{}

	if completed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d completed%s", colorGreen, completed, colorReset))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%s%d running%s", colorYellow, running, colorReset))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d failed%s", colorRed, failed, colorReset))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", pending))
	}
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", blocked))
	}

	if len(parts) == 0 {
		fmt.Printf("0 features")
	} else {
		fmt.Printf("%s", strings.Join(parts, ", "))
	}

	fmt.Printf(" (%d total)\n", total)
}
