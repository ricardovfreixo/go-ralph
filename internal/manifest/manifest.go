package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/parser"
	"github.com/vx/ralph-go/internal/usage"
)

const DefaultMaxDepth = 3

type EscalationConfig struct {
	Enabled            bool     `json:"enabled"`
	ErrorThreshold     int      `json:"error_threshold,omitempty"`
	EscalateKeywords   []string `json:"escalate_keywords,omitempty"`
	DeescalateKeywords []string `json:"deescalate_keywords,omitempty"`
}

type Manifest struct {
	mu           sync.RWMutex
	path         string
	Source       string            `json:"source"`
	Title        string            `json:"title"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
	Features     []ManifestFeature `json:"features"`
	BudgetTokens int64             `json:"budget_tokens,omitempty"`
	BudgetUSD    float64           `json:"budget_usd,omitempty"`
	MaxDepth     int               `json:"max_depth,omitempty"`  // Max recursion depth (default: 3)
	Escalation   *EscalationConfig `json:"escalation,omitempty"` // Model escalation configuration
}

type ManifestFeature struct {
	ID           string            `json:"id"`
	Dir          string            `json:"dir"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	DependsOn    []string          `json:"depends_on"`
	Execution    string            `json:"execution"`
	Model        string            `json:"model"`
	Usage        *usage.TokenUsage `json:"usage,omitempty"`
	BudgetTokens int64             `json:"budget_tokens,omitempty"`
	BudgetUSD    float64           `json:"budget_usd,omitempty"`

	// Recursive feature fields (RLM support)
	ParentID      string   `json:"parent_id,omitempty"`      // Empty for root features
	Depth         int      `json:"depth,omitempty"`          // 0 for root features
	Children      []string `json:"children,omitempty"`       // Child feature IDs
	ContextBudget int64    `json:"context_budget,omitempty"` // Tokens available for context
}

var dependsRegex = regexp.MustCompile(`(?i)^depends:\s*(.+)$`)

func New(source, title string) *Manifest {
	return &Manifest{
		Source:   source,
		Title:    title,
		Created:  time.Now(),
		Updated:  time.Now(),
		Features: make([]ManifestFeature, 0),
	}
}

func Load(prdDir string) (*Manifest, error) {
	manifestPath := filepath.Join(prdDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	m.path = manifestPath
	return &m, nil
}

func (m *Manifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.path == "" {
		return fmt.Errorf("manifest path not set")
	}

	m.Updated = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

func (m *Manifest) SetPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.path = path
}

func (m *Manifest) GetPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.path
}

func (m *Manifest) UpdateFeatureStatus(id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.Features {
		if m.Features[i].ID == id {
			m.Features[i].Status = status
			m.Updated = time.Now()
			return nil
		}
	}
	return fmt.Errorf("feature not found: %s", id)
}

func (m *Manifest) UpdateFeatureUsage(id string, u *usage.TokenUsage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.Features {
		if m.Features[i].ID == id {
			if u != nil {
				snapshot := u.Snapshot()
				m.Features[i].Usage = &snapshot
			}
			m.Updated = time.Now()
			return nil
		}
	}
	return fmt.Errorf("feature not found: %s", id)
}

func (m *Manifest) GetFeature(id string) *ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.Features {
		if m.Features[i].ID == id {
			return &m.Features[i]
		}
	}
	return nil
}

func (m *Manifest) GetFeatureByTitle(title string) *ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	for i := range m.Features {
		if strings.ToLower(m.Features[i].Title) == normalizedTitle {
			return &m.Features[i]
		}
	}
	return nil
}

func (m *Manifest) AllFeatures() []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ManifestFeature, len(m.Features))
	copy(result, m.Features)
	return result
}

func GenerateFromPRD(prd *parser.PRD, sourcePath string) (*Manifest, error) {
	manifest := New(filepath.Base(sourcePath), prd.Title)
	manifest.BudgetTokens = prd.BudgetTokens
	manifest.BudgetUSD = prd.BudgetUSD

	for i, feature := range prd.Features {
		id := fmt.Sprintf("%02d", i+1)
		dirName := fmt.Sprintf("%s-%s", id, sanitizeDirName(feature.Title))

		deps := ParseDependencies(feature.RawContent, feature.Description)

		mf := ManifestFeature{
			ID:           id,
			Dir:          dirName,
			Title:        feature.Title,
			Status:       "pending",
			DependsOn:    deps,
			Execution:    feature.ExecutionMode,
			Model:        feature.Model,
			BudgetTokens: feature.BudgetTokens,
			BudgetUSD:    feature.BudgetUSD,
		}
		manifest.Features = append(manifest.Features, mf)
	}

	return manifest, nil
}

func ParseDependencies(rawContent, description string) []string {
	deps := []string{}

	content := rawContent
	if content == "" {
		content = description
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := dependsRegex.FindStringSubmatch(line); matches != nil {
			depStr := strings.TrimSpace(matches[1])
			for _, dep := range strings.Split(depStr, ",") {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					deps = append(deps, dep)
				}
			}
		}
	}

	return deps
}

func (m *Manifest) ResolveDependencyID(dep string) string {
	dep = strings.TrimSpace(dep)

	if _, err := strconv.Atoi(dep); err == nil {
		id := fmt.Sprintf("%02d", mustAtoi(dep))
		if m.GetFeature(id) != nil {
			return id
		}
		if m.GetFeature(dep) != nil {
			return dep
		}
	}

	if f := m.GetFeatureByTitle(dep); f != nil {
		return f.ID
	}

	return dep
}

func (m *Manifest) ResolveDependencies() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.Features {
		resolved := make([]string, 0, len(m.Features[i].DependsOn))
		for _, dep := range m.Features[i].DependsOn {
			resolvedID := m.resolveDepIDUnlocked(dep)
			resolved = append(resolved, resolvedID)
		}
		m.Features[i].DependsOn = resolved
	}
}

func (m *Manifest) resolveDepIDUnlocked(dep string) string {
	dep = strings.TrimSpace(dep)

	if _, err := strconv.Atoi(dep); err == nil {
		id := fmt.Sprintf("%02d", mustAtoi(dep))
		for _, f := range m.Features {
			if f.ID == id {
				return id
			}
		}
		for _, f := range m.Features {
			if f.ID == dep {
				return dep
			}
		}
	}

	normalizedDep := strings.ToLower(dep)
	for _, f := range m.Features {
		if strings.ToLower(f.Title) == normalizedDep {
			return f.ID
		}
	}

	return dep
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func sanitizeDirName(title string) string {
	title = strings.ToLower(title)

	prefixRegex := regexp.MustCompile(`^feature\s*\d*:\s*`)
	title = prefixRegex.ReplaceAllString(title, "")

	title = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, title)

	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}
	title = strings.Trim(title, "-")

	if len(title) > 50 {
		title = title[:50]
		title = strings.TrimRight(title, "-")
	}

	return title
}

func (m *Manifest) IsDependencySatisfied(featureID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var feature *ManifestFeature
	for i := range m.Features {
		if m.Features[i].ID == featureID {
			feature = &m.Features[i]
			break
		}
	}

	if feature == nil {
		return false
	}

	if len(feature.DependsOn) == 0 {
		return true
	}

	for _, depID := range feature.DependsOn {
		depFeature := m.getFeatureUnlocked(depID)
		if depFeature == nil || depFeature.Status != "completed" {
			return false
		}
	}

	return true
}

func (m *Manifest) getFeatureUnlocked(id string) *ManifestFeature {
	for i := range m.Features {
		if m.Features[i].ID == id {
			return &m.Features[i]
		}
	}
	return nil
}

func (m *Manifest) GetPendingDependencies(featureID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var feature *ManifestFeature
	for i := range m.Features {
		if m.Features[i].ID == featureID {
			feature = &m.Features[i]
			break
		}
	}

	if feature == nil {
		return nil
	}

	pending := []string{}
	for _, depID := range feature.DependsOn {
		depFeature := m.getFeatureUnlocked(depID)
		if depFeature == nil || depFeature.Status != "completed" {
			pending = append(pending, depID)
		}
	}

	return pending
}

func (m *Manifest) GetSummary() (total, completed, running, failed, pending, blocked int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total = len(m.Features)
	for _, feature := range m.Features {
		switch feature.Status {
		case "completed":
			completed++
		case "running":
			running++
		case "failed":
			failed++
		default:
			if m.isDependencySatisfiedUnlocked(feature.ID) {
				pending++
			} else {
				blocked++
			}
		}
	}
	return
}

func (m *Manifest) isDependencySatisfiedUnlocked(featureID string) bool {
	var feature *ManifestFeature
	for i := range m.Features {
		if m.Features[i].ID == featureID {
			feature = &m.Features[i]
			break
		}
	}

	if feature == nil {
		return false
	}

	if len(feature.DependsOn) == 0 {
		return true
	}

	for _, depID := range feature.DependsOn {
		depFeature := m.getFeatureUnlocked(depID)
		if depFeature == nil || depFeature.Status != "completed" {
			return false
		}
	}

	return true
}

// HasGlobalBudget returns true if a global budget limit is set
func (m *Manifest) HasGlobalBudget() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.BudgetTokens > 0 || m.BudgetUSD > 0
}

// GetGlobalBudget returns the global budget settings
func (m *Manifest) GetGlobalBudget() (tokens int64, usd float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.BudgetTokens, m.BudgetUSD
}

// GetFeatureBudget returns the budget for a specific feature
func (m *Manifest) GetFeatureBudget(id string) (tokens int64, usd float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, f := range m.Features {
		if f.ID == id {
			return f.BudgetTokens, f.BudgetUSD
		}
	}
	return 0, 0
}

// GetTotalUsage calculates total token usage across all features
func (m *Manifest) GetTotalUsage() *usage.TokenUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := usage.New()
	for _, f := range m.Features {
		if f.Usage != nil {
			total.Add(f.Usage)
		}
	}
	return total
}

// GetMaxDepth returns the configured max depth, defaulting to DefaultMaxDepth
func (m *Manifest) GetMaxDepth() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.MaxDepth <= 0 {
		return DefaultMaxDepth
	}
	return m.MaxDepth
}

// SetMaxDepth sets the max recursion depth
func (m *Manifest) SetMaxDepth(depth int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MaxDepth = depth
}

// IsRootFeature returns true if the feature has no parent
func (f *ManifestFeature) IsRootFeature() bool {
	return f.ParentID == ""
}

// HasChildren returns true if the feature has spawned sub-features
func (f *ManifestFeature) HasChildren() bool {
	return len(f.Children) > 0
}

// AddSubFeature adds a new sub-feature to the manifest and links it to its parent
func (m *Manifest) AddSubFeature(parentID string, feature ManifestFeature) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	parentIdx := -1
	for i := range m.Features {
		if m.Features[i].ID == parentID {
			parentIdx = i
			break
		}
	}
	if parentIdx == -1 {
		return fmt.Errorf("parent feature not found: %s", parentID)
	}

	parent := &m.Features[parentIdx]

	maxDepth := m.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	if parent.Depth+1 > maxDepth {
		return fmt.Errorf("max depth exceeded: parent at depth %d, max is %d", parent.Depth, maxDepth)
	}

	feature.ParentID = parentID
	feature.Depth = parent.Depth + 1

	if feature.ContextBudget == 0 && parent.ContextBudget > 0 {
		feature.ContextBudget = parent.ContextBudget / 2
	}

	m.Features = append(m.Features, feature)
	m.Features[parentIdx].Children = append(m.Features[parentIdx].Children, feature.ID)
	m.Updated = time.Now()

	return nil
}

// GetChildren returns the child features of a given feature
func (m *Manifest) GetChildren(parentID string) []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	parent := m.getFeatureUnlocked(parentID)
	if parent == nil {
		return nil
	}

	children := make([]ManifestFeature, 0, len(parent.Children))
	for _, childID := range parent.Children {
		if child := m.getFeatureUnlocked(childID); child != nil {
			children = append(children, *child)
		}
	}
	return children
}

// GetParent returns the parent feature, or nil if this is a root feature
func (m *Manifest) GetParent(featureID string) *ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feature := m.getFeatureUnlocked(featureID)
	if feature == nil || feature.ParentID == "" {
		return nil
	}
	return m.getFeatureUnlocked(feature.ParentID)
}

// GetRootFeatures returns all features that have no parent
func (m *Manifest) GetRootFeatures() []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roots := make([]ManifestFeature, 0)
	for _, f := range m.Features {
		if f.ParentID == "" {
			roots = append(roots, f)
		}
	}
	return roots
}

// GetDescendants returns all descendants of a feature (children, grandchildren, etc.)
func (m *Manifest) GetDescendants(featureID string) []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var descendants []ManifestFeature
	m.collectDescendantsUnlocked(featureID, &descendants)
	return descendants
}

func (m *Manifest) collectDescendantsUnlocked(featureID string, result *[]ManifestFeature) {
	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return
	}

	for _, childID := range feature.Children {
		if child := m.getFeatureUnlocked(childID); child != nil {
			*result = append(*result, *child)
			m.collectDescendantsUnlocked(childID, result)
		}
	}
}

// GetFeatureWithDescendants returns a feature and all its descendants
func (m *Manifest) GetFeatureWithDescendants(featureID string) []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return nil
	}

	result := []ManifestFeature{*feature}
	m.collectDescendantsUnlocked(featureID, &result)
	return result
}

// GetAncestors returns all ancestors of a feature (parent, grandparent, etc.)
func (m *Manifest) GetAncestors(featureID string) []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ancestors []ManifestFeature
	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return nil
	}

	currentID := feature.ParentID
	for currentID != "" {
		parent := m.getFeatureUnlocked(currentID)
		if parent == nil {
			break
		}
		ancestors = append(ancestors, *parent)
		currentID = parent.ParentID
	}
	return ancestors
}

// GetTreeUsage calculates total token usage for a feature and all its descendants
func (m *Manifest) GetTreeUsage(featureID string) *usage.TokenUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := usage.New()
	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return total
	}

	if feature.Usage != nil {
		total.Add(feature.Usage)
	}
	m.collectTreeUsageUnlocked(featureID, total)
	return total
}

func (m *Manifest) collectTreeUsageUnlocked(featureID string, total *usage.TokenUsage) {
	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return
	}

	for _, childID := range feature.Children {
		if child := m.getFeatureUnlocked(childID); child != nil {
			if child.Usage != nil {
				total.Add(child.Usage)
			}
			m.collectTreeUsageUnlocked(childID, total)
		}
	}
}

// CanSpawnChild returns true if the feature can spawn a child (not at max depth)
func (m *Manifest) CanSpawnChild(featureID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feature := m.getFeatureUnlocked(featureID)
	if feature == nil {
		return false
	}

	maxDepth := m.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return feature.Depth < maxDepth
}

// GetEscalationConfig returns the escalation configuration
func (m *Manifest) GetEscalationConfig() *EscalationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Escalation
}

// SetEscalationConfig sets the escalation configuration
func (m *Manifest) SetEscalationConfig(config *EscalationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Escalation = config
	m.Updated = time.Now()
}

// IsEscalationEnabled returns true if model escalation is enabled
func (m *Manifest) IsEscalationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Escalation != nil && m.Escalation.Enabled
}

// UpdateFeatureModel updates the model for a feature
func (m *Manifest) UpdateFeatureModel(id, model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.Features {
		if m.Features[i].ID == id {
			m.Features[i].Model = model
			m.Updated = time.Now()
			return nil
		}
	}
	return fmt.Errorf("feature not found: %s", id)
}
