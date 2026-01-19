package escalation

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/vx/ralph-go/internal/logger"
)

const (
	ModelHaiku  = "haiku"
	ModelSonnet = "sonnet"
	ModelOpus   = "opus"
)

type Reason string

const (
	ReasonInitial          Reason = "initial"
	ReasonRepeatedErrors   Reason = "repeated_errors"
	ReasonExplicitRequest  Reason = "explicit_request"
	ReasonKeywordMatch     Reason = "keyword_match"
	ReasonDeescalate       Reason = "deescalate"
	ReasonComplexity       Reason = "complexity"
	ReasonTestFailure      Reason = "test_failure"
	ReasonArchitectural    Reason = "architectural"
	ReasonConfiguredByUser Reason = "configured_by_user"
)

type ModelSwitch struct {
	Timestamp time.Time `json:"timestamp"`
	FromModel string    `json:"from_model"`
	ToModel   string    `json:"to_model"`
	Reason    Reason    `json:"reason"`
	Details   string    `json:"details,omitempty"`
}

type TriggerConfig struct {
	ErrorThreshold     int      `json:"error_threshold"`
	EscalateKeywords   []string `json:"escalate_keywords"`
	DeescalateKeywords []string `json:"deescalate_keywords"`
	Enabled            bool     `json:"enabled"`
}

func DefaultTriggerConfig() TriggerConfig {
	return TriggerConfig{
		ErrorThreshold: 2,
		EscalateKeywords: []string{
			"architect", "architecture", "architectural",
			"design pattern", "system design", "api design",
			"refactor", "refactoring",
			"trade-off", "tradeoff",
			"database schema", "data model",
			"complex", "complexity",
		},
		DeescalateKeywords: []string{
			"test", "tests", "testing",
			"format", "formatting", "lint", "linting",
			"typo", "typos", "spelling",
			"comment", "comments", "documentation",
			"simple", "trivial", "minor",
		},
		Enabled: true,
	}
}

type Tracker struct {
	mu           sync.RWMutex
	featureID    string
	currentModel string
	initialModel string
	switches     []ModelSwitch
	errorCount   int
	testFailures int
	config       TriggerConfig
}

func NewTracker(featureID, initialModel string, config TriggerConfig) *Tracker {
	if initialModel == "" || initialModel == "auto" {
		initialModel = ModelSonnet
	}
	t := &Tracker{
		featureID:    featureID,
		currentModel: initialModel,
		initialModel: initialModel,
		config:       config,
		switches: []ModelSwitch{{
			Timestamp: time.Now(),
			FromModel: "",
			ToModel:   initialModel,
			Reason:    ReasonInitial,
			Details:   "initial model selection",
		}},
	}
	return t
}

func (t *Tracker) CurrentModel() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentModel
}

func (t *Tracker) Switches() []ModelSwitch {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]ModelSwitch, len(t.switches))
	copy(result, t.switches)
	return result
}

func (t *Tracker) ErrorCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.errorCount
}

type StreamMessage struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Content   string          `json:"content,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
}

func (t *Tracker) ProcessLine(line string) (changed bool, newModel string) {
	if !t.config.Enabled {
		return false, t.CurrentModel()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var msg StreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return false, t.currentModel
	}

	switch msg.Type {
	case "tool_result":
		if msg.IsError {
			return t.handleErrorUnlocked(msg)
		}
		return t.checkContentEscalationUnlocked(msg.Result)
	case "assistant":
		content := t.extractContentUnlocked(msg)
		return t.checkContentEscalationUnlocked(content)
	case "error":
		return t.handleErrorUnlocked(msg)
	}

	return false, t.currentModel
}

func (t *Tracker) handleErrorUnlocked(msg StreamMessage) (bool, string) {
	t.errorCount++

	if t.isTestOrBuildError(msg.Result) {
		t.testFailures++
	}

	if t.errorCount >= t.config.ErrorThreshold {
		targetModel := t.getNextEscalationModel()
		if targetModel != t.currentModel {
			return t.switchModelUnlocked(targetModel, ReasonRepeatedErrors,
				"%d errors at %s level", t.errorCount, t.currentModel)
		}
	}

	return false, t.currentModel
}

func (t *Tracker) checkContentEscalationUnlocked(content string) (bool, string) {
	if content == "" {
		return false, t.currentModel
	}

	lower := strings.ToLower(content)

	if changed, model := t.checkExplicitRequestUnlocked(lower); changed {
		return changed, model
	}

	if changed, model := t.checkKeywordEscalationUnlocked(lower); changed {
		return changed, model
	}

	if changed, model := t.checkKeywordDeescalationUnlocked(lower); changed {
		return changed, model
	}

	return false, t.currentModel
}

func (t *Tracker) checkExplicitRequestUnlocked(content string) (bool, string) {
	explicitPatterns := []struct {
		pattern string
		model   string
	}{
		{"need opus", ModelOpus},
		{"needs opus", ModelOpus},
		{"require opus", ModelOpus},
		{"requires opus", ModelOpus},
		{"switch to opus", ModelOpus},
		{"escalate to opus", ModelOpus},
		{"this needs opus", ModelOpus},
		{"need sonnet", ModelSonnet},
		{"needs sonnet", ModelSonnet},
		{"require sonnet", ModelSonnet},
		{"requires sonnet", ModelSonnet},
		{"switch to sonnet", ModelSonnet},
		{"escalate to sonnet", ModelSonnet},
		{"need haiku", ModelHaiku},
		{"needs haiku", ModelHaiku},
		{"switch to haiku", ModelHaiku},
	}

	for _, p := range explicitPatterns {
		if strings.Contains(content, p.pattern) {
			if p.model != t.currentModel {
				return t.switchModelUnlocked(p.model, ReasonExplicitRequest,
					"explicit request: '%s'", p.pattern)
			}
		}
	}

	return false, t.currentModel
}

func (t *Tracker) checkKeywordEscalationUnlocked(content string) (bool, string) {
	if t.currentModel == ModelOpus {
		return false, t.currentModel
	}

	matchCount := 0
	var matchedKeywords []string

	for _, keyword := range t.config.EscalateKeywords {
		if strings.Contains(content, strings.ToLower(keyword)) {
			matchCount++
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}

	if matchCount >= 2 || t.containsArchitecturalPattern(content) {
		targetModel := t.getNextEscalationModel()
		if targetModel != t.currentModel {
			return t.switchModelUnlocked(targetModel, ReasonKeywordMatch,
				"escalation keywords detected: %v", matchedKeywords)
		}
	}

	return false, t.currentModel
}

func (t *Tracker) checkKeywordDeescalationUnlocked(content string) (bool, string) {
	if t.currentModel == ModelHaiku {
		return false, t.currentModel
	}

	if t.errorCount > 0 {
		return false, t.currentModel
	}

	matchCount := 0
	var matchedKeywords []string

	for _, keyword := range t.config.DeescalateKeywords {
		if strings.Contains(content, strings.ToLower(keyword)) {
			matchCount++
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}

	if matchCount >= 2 {
		targetModel := t.getDeescalationModel()
		if targetModel != t.currentModel {
			return t.switchModelUnlocked(targetModel, ReasonDeescalate,
				"de-escalation keywords detected: %v", matchedKeywords)
		}
	}

	return false, t.currentModel
}

var architecturalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)architect(ure|ural)?`),
	regexp.MustCompile(`(?i)design\s+(pattern|decision|choice)`),
	regexp.MustCompile(`(?i)refactor(ing)?\s+(strategy|approach|plan)`),
	regexp.MustCompile(`(?i)trade-?off`),
	regexp.MustCompile(`(?i)system\s+design`),
	regexp.MustCompile(`(?i)api\s+design`),
	regexp.MustCompile(`(?i)database\s+schema`),
	regexp.MustCompile(`(?i)data\s+model`),
	regexp.MustCompile(`(?i)major\s+refactor`),
}

func (t *Tracker) containsArchitecturalPattern(content string) bool {
	for _, pattern := range architecturalPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func (t *Tracker) getNextEscalationModel() string {
	switch t.currentModel {
	case ModelHaiku:
		return ModelSonnet
	case ModelSonnet:
		return ModelOpus
	default:
		return t.currentModel
	}
}

func (t *Tracker) getDeescalationModel() string {
	switch t.currentModel {
	case ModelOpus:
		return ModelSonnet
	case ModelSonnet:
		return ModelHaiku
	default:
		return t.currentModel
	}
}

func (t *Tracker) switchModelUnlocked(model string, reason Reason, detailsFmt string, args ...any) (bool, string) {
	oldModel := t.currentModel
	t.currentModel = model

	details := ""
	if len(args) > 0 {
		details = sprintf(detailsFmt, args...)
	} else {
		details = detailsFmt
	}

	sw := ModelSwitch{
		Timestamp: time.Now(),
		FromModel: oldModel,
		ToModel:   model,
		Reason:    reason,
		Details:   details,
	}
	t.switches = append(t.switches, sw)

	logger.Info("escalation", "Model switch",
		"featureID", t.featureID,
		"from", oldModel,
		"to", model,
		"reason", string(reason),
		"details", details)

	return true, model
}

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return strings.ReplaceAll(strings.ReplaceAll(
		strings.ReplaceAll(format, "%d", intToStr(args)),
		"%s", strArg(args)),
		"%v", anyToStr(args))
}

func intToStr(args []any) string {
	for _, a := range args {
		if v, ok := a.(int); ok {
			return string(rune('0' + v%10))
		}
	}
	return "?"
}

func strArg(args []any) string {
	for _, a := range args {
		if v, ok := a.(string); ok {
			return v
		}
	}
	return ""
}

func anyToStr(args []any) string {
	for _, a := range args {
		switch v := a.(type) {
		case []string:
			return strings.Join(v, ", ")
		case string:
			return v
		}
	}
	return ""
}

func (t *Tracker) extractContentUnlocked(msg StreamMessage) string {
	if msg.Content != "" {
		return msg.Content
	}
	if len(msg.Message) == 0 {
		return ""
	}
	var content struct {
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(msg.Message, &content); err == nil {
		if content.Content != "" {
			return content.Content
		}
		return content.Text
	}
	return ""
}

func (t *Tracker) isTestOrBuildError(result string) bool {
	lower := strings.ToLower(result)
	return strings.Contains(lower, "test failed") ||
		strings.Contains(lower, "--- fail") ||
		strings.Contains(lower, "build failed") ||
		strings.Contains(lower, "compilation failed") ||
		strings.Contains(lower, "compile error") ||
		strings.Contains(lower, "syntax error")
}

func (t *Tracker) ForceModel(model, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == t.currentModel {
		return
	}

	t.switchModelUnlocked(model, ReasonConfiguredByUser, reason)
}

func (t *Tracker) ResetErrors() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorCount = 0
	t.testFailures = 0
}

func (t *Tracker) ShouldRestart() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.switches) <= 1 {
		return false
	}
	lastSwitch := t.switches[len(t.switches)-1]
	return lastSwitch.Reason != ReasonInitial
}

func (t *Tracker) LastSwitch() *ModelSwitch {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.switches) == 0 {
		return nil
	}
	sw := t.switches[len(t.switches)-1]
	return &sw
}

type Manager struct {
	mu       sync.RWMutex
	trackers map[string]*Tracker
	config   TriggerConfig
}

func NewManager() *Manager {
	return &Manager{
		trackers: make(map[string]*Tracker),
		config:   DefaultTriggerConfig(),
	}
}

func NewManagerWithConfig(config TriggerConfig) *Manager {
	return &Manager{
		trackers: make(map[string]*Tracker),
		config:   config,
	}
}

func (m *Manager) SetConfig(config TriggerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

func (m *Manager) GetConfig() TriggerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *Manager) Register(featureID, initialModel string) *Tracker {
	m.mu.Lock()
	defer m.mu.Unlock()

	tracker := NewTracker(featureID, initialModel, m.config)
	m.trackers[featureID] = tracker

	logger.Info("escalation", "Tracker registered",
		"featureID", featureID,
		"initialModel", initialModel)

	return tracker
}

func (m *Manager) Get(featureID string) *Tracker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trackers[featureID]
}

func (m *Manager) Remove(featureID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.trackers, featureID)
}

func (m *Manager) ProcessLine(featureID, line string) (changed bool, newModel string) {
	tracker := m.Get(featureID)
	if tracker == nil {
		return false, ""
	}
	return tracker.ProcessLine(line)
}

func (m *Manager) GetCurrentModel(featureID string) string {
	tracker := m.Get(featureID)
	if tracker == nil {
		return ""
	}
	return tracker.CurrentModel()
}

func (m *Manager) GetAllSwitches(featureID string) []ModelSwitch {
	tracker := m.Get(featureID)
	if tracker == nil {
		return nil
	}
	return tracker.Switches()
}

func SelectInitialModel(taskCount int, keywords []string) string {
	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		if strings.Contains(lower, "architect") ||
			strings.Contains(lower, "design") ||
			strings.Contains(lower, "refactor") {
			return ModelSonnet
		}
	}

	if taskCount <= 2 {
		return ModelHaiku
	}
	if taskCount <= 5 {
		return ModelSonnet
	}
	return ModelSonnet
}

func ShouldDeescalateForTask(taskTitle string) bool {
	lower := strings.ToLower(taskTitle)
	deescalatePatterns := []string{
		"test", "format", "lint", "typo", "comment",
		"documentation", "simple", "trivial", "minor",
	}
	for _, pattern := range deescalatePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func ShouldEscalateForTask(taskTitle string) bool {
	lower := strings.ToLower(taskTitle)
	escalatePatterns := []string{
		"architect", "design", "refactor", "complex",
		"system", "api design", "database", "schema",
	}
	for _, pattern := range escalatePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
