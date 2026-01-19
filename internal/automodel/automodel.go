package automodel

import (
	"encoding/json"
	"fmt"
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

type EscalationReason string

const (
	ReasonInitial            EscalationReason = "initial"
	ReasonToolError          EscalationReason = "tool_error"
	ReasonTestFailure        EscalationReason = "test_failure"
	ReasonComplexityDetected EscalationReason = "complexity_detected"
	ReasonArchitectural      EscalationReason = "architectural"
	ReasonExplicitRequest    EscalationReason = "explicit_request"
	ReasonMultipleErrors     EscalationReason = "multiple_errors"
	ReasonDebugging          EscalationReason = "debugging"
	ReasonDeescalate         EscalationReason = "deescalate"
)

type Config struct {
	ErrorThreshold     int
	EscalateKeywords   []string
	DeescalateKeywords []string
	Enabled            bool
}

func DefaultConfig() Config {
	return Config{
		ErrorThreshold: 2,
		EscalateKeywords: []string{
			"architect", "architecture", "architectural",
			"design pattern", "system design", "api design",
			"refactor", "refactoring",
			"trade-off", "tradeoff",
			"database schema", "data model",
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

type ModelSwitch struct {
	Timestamp  time.Time        `json:"timestamp"`
	FromModel  string           `json:"from_model"`
	ToModel    string           `json:"to_model"`
	Reason     EscalationReason `json:"reason"`
	Details    string           `json:"details,omitempty"`
	TokenCount int64            `json:"token_count,omitempty"`
}

type Selector struct {
	mu            sync.RWMutex
	featureID     string
	currentModel  string
	switches      []ModelSwitch
	errorCount    int
	testFailCount int
	isLeafTask    bool
	taskCount     int
	config        Config
}

func NewSelector(featureID string, isLeafTask bool, taskCount int) *Selector {
	return NewSelectorWithConfig(featureID, isLeafTask, taskCount, DefaultConfig())
}

func NewSelectorWithConfig(featureID string, isLeafTask bool, taskCount int, config Config) *Selector {
	s := &Selector{
		featureID:  featureID,
		isLeafTask: isLeafTask,
		taskCount:  taskCount,
		config:     config,
	}
	s.currentModel = s.selectInitialModel()
	s.switches = append(s.switches, ModelSwitch{
		Timestamp: time.Now(),
		FromModel: "",
		ToModel:   s.currentModel,
		Reason:    ReasonInitial,
		Details:   s.initialModelDetails(),
	})
	return s
}

func (s *Selector) selectInitialModel() string {
	if s.isLeafTask || s.taskCount <= 2 {
		return ModelHaiku
	}
	if s.taskCount <= 5 {
		return ModelHaiku
	}
	return ModelSonnet
}

func (s *Selector) initialModelDetails() string {
	if s.isLeafTask {
		return "leaf task - starting with haiku"
	}
	if s.taskCount <= 2 {
		return "simple task (<=2 tasks) - starting with haiku"
	}
	if s.taskCount <= 5 {
		return "moderate task (<=5 tasks) - starting with haiku"
	}
	return "complex task (>5 tasks) - starting with sonnet"
}

func (s *Selector) CurrentModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentModel
}

func (s *Selector) Switches() []ModelSwitch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ModelSwitch, len(s.switches))
	copy(result, s.switches)
	return result
}

func (s *Selector) ProcessLine(line string) (modelChanged bool, newModel string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var msg StreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return false, s.currentModel
	}

	switch msg.Type {
	case "tool_result":
		if msg.IsError {
			return s.handleToolError(msg)
		}
		return s.checkToolResultComplexity(msg)
	case "assistant":
		return s.checkAssistantComplexity(msg)
	case "error":
		return s.handleError(msg)
	}

	return false, s.currentModel
}

type StreamMessage struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Content   string          `json:"content,omitempty"`
}

func (s *Selector) handleToolError(msg StreamMessage) (bool, string) {
	s.errorCount++

	threshold := s.config.ErrorThreshold
	if threshold <= 0 {
		threshold = 2
	}

	if s.errorCount >= threshold && s.currentModel == ModelHaiku {
		details := fmt.Sprintf("%d+ tool errors with haiku", s.errorCount)
		return s.escalateTo(ModelSonnet, ReasonMultipleErrors, details)
	}

	if isCompilationError(msg.Result) || isTestError(msg.Result) {
		s.testFailCount++
		if s.testFailCount >= threshold && s.currentModel == ModelHaiku {
			return s.escalateTo(ModelSonnet, ReasonTestFailure, "repeated test/build failures")
		}
		if s.testFailCount >= threshold && s.currentModel == ModelSonnet {
			return s.escalateTo(ModelOpus, ReasonTestFailure, "repeated test/build failures at sonnet level")
		}
	}

	return false, s.currentModel
}

func (s *Selector) handleError(msg StreamMessage) (bool, string) {
	s.errorCount++

	if s.errorCount >= 2 && s.currentModel == ModelHaiku {
		return s.escalateTo(ModelSonnet, ReasonMultipleErrors, "multiple errors encountered")
	}

	return false, s.currentModel
}

func (s *Selector) checkToolResultComplexity(msg StreamMessage) (bool, string) {
	if s.currentModel == ModelOpus {
		return false, s.currentModel
	}

	result := msg.Result

	if isArchitecturalContent(result) {
		if s.currentModel == ModelHaiku {
			return s.escalateTo(ModelSonnet, ReasonArchitectural, "architectural complexity detected")
		}
		if s.currentModel == ModelSonnet && len(result) > 5000 {
			return s.escalateTo(ModelOpus, ReasonArchitectural, "major architectural decisions needed")
		}
	}

	if isDebuggingScenario(result) && s.currentModel != ModelOpus {
		if s.currentModel == ModelHaiku {
			return s.escalateTo(ModelSonnet, ReasonDebugging, "debugging scenario detected")
		}
		if containsComplexDebugging(result) {
			return s.escalateTo(ModelOpus, ReasonDebugging, "complex debugging required")
		}
	}

	return false, s.currentModel
}

func (s *Selector) checkAssistantComplexity(msg StreamMessage) (bool, string) {
	content := extractContent(msg)

	if containsModelEscalationRequest(content) {
		targetModel := extractRequestedModel(content)
		if targetModel != "" && targetModel != s.currentModel {
			if isHigherTier(targetModel, s.currentModel) {
				return s.escalateTo(targetModel, ReasonExplicitRequest, "explicit escalation in output")
			}
			if isLowerTier(targetModel, s.currentModel) && s.errorCount == 0 {
				return s.deescalateTo(targetModel, "explicit de-escalation in output")
			}
		}
	}

	if s.currentModel != ModelOpus {
		if s.containsEscalationKeywords(content) && s.currentModel == ModelHaiku {
			return s.escalateTo(ModelSonnet, ReasonArchitectural, "escalation keywords detected")
		}
	}

	if s.currentModel != ModelHaiku && s.errorCount == 0 {
		if changed, model := s.checkDeescalation(content); changed {
			return changed, model
		}
	}

	return false, s.currentModel
}

func (s *Selector) containsEscalationKeywords(content string) bool {
	lower := strings.ToLower(content)

	for _, keyword := range s.config.EscalateKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}

	return isArchitecturalContent(content)
}

func (s *Selector) checkDeescalation(content string) (bool, string) {
	if s.currentModel == ModelHaiku {
		return false, s.currentModel
	}

	if s.errorCount > 0 {
		return false, s.currentModel
	}

	lower := strings.ToLower(content)
	matchCount := 0
	var matchedKeywords []string

	for _, keyword := range s.config.DeescalateKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			matchCount++
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}

	if matchCount >= 2 {
		targetModel := s.getDeescalationTarget()
		if targetModel != s.currentModel {
			details := "de-escalation keywords: " + strings.Join(matchedKeywords, ", ")
			return s.deescalateTo(targetModel, details)
		}
	}

	return false, s.currentModel
}

func (s *Selector) getDeescalationTarget() string {
	switch s.currentModel {
	case ModelOpus:
		return ModelSonnet
	case ModelSonnet:
		return ModelHaiku
	default:
		return s.currentModel
	}
}

func (s *Selector) deescalateTo(model string, details string) (bool, string) {
	if !isLowerTier(model, s.currentModel) {
		return false, s.currentModel
	}

	oldModel := s.currentModel
	s.currentModel = model

	sw := ModelSwitch{
		Timestamp: time.Now(),
		FromModel: oldModel,
		ToModel:   model,
		Reason:    ReasonDeescalate,
		Details:   details,
	}
	s.switches = append(s.switches, sw)

	logger.Info("automodel", "Model de-escalated",
		"featureID", s.featureID,
		"from", oldModel,
		"to", model,
		"details", details)

	return true, model
}

func isLowerTier(target, current string) bool {
	tiers := map[string]int{
		ModelHaiku:  0,
		ModelSonnet: 1,
		ModelOpus:   2,
	}
	return tiers[target] < tiers[current]
}

func (s *Selector) escalateTo(model string, reason EscalationReason, details string) (bool, string) {
	if !isHigherTier(model, s.currentModel) {
		return false, s.currentModel
	}

	oldModel := s.currentModel
	s.currentModel = model

	sw := ModelSwitch{
		Timestamp: time.Now(),
		FromModel: oldModel,
		ToModel:   model,
		Reason:    reason,
		Details:   details,
	}
	s.switches = append(s.switches, sw)

	logger.Info("automodel", "Model escalated",
		"featureID", s.featureID,
		"from", oldModel,
		"to", model,
		"reason", string(reason),
		"details", details)

	return true, model
}

func (s *Selector) ForceModel(model string, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if model == s.currentModel {
		return
	}

	oldModel := s.currentModel
	s.currentModel = model

	sw := ModelSwitch{
		Timestamp: time.Now(),
		FromModel: oldModel,
		ToModel:   model,
		Reason:    ReasonExplicitRequest,
		Details:   reason,
	}
	s.switches = append(s.switches, sw)

	logger.Info("automodel", "Model forced",
		"featureID", s.featureID,
		"from", oldModel,
		"to", model,
		"reason", reason)
}

func isHigherTier(target, current string) bool {
	tiers := map[string]int{
		ModelHaiku:  0,
		ModelSonnet: 1,
		ModelOpus:   2,
	}
	return tiers[target] > tiers[current]
}

var architecturalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)architect(ure|ural)`),
	regexp.MustCompile(`(?i)design\s+(pattern|decision|choice)`),
	regexp.MustCompile(`(?i)refactor(ing)?\s+(strategy|approach)`),
	regexp.MustCompile(`(?i)trade-?off`),
	regexp.MustCompile(`(?i)system\s+design`),
	regexp.MustCompile(`(?i)interface\s+design`),
	regexp.MustCompile(`(?i)api\s+design`),
	regexp.MustCompile(`(?i)database\s+schema`),
	regexp.MustCompile(`(?i)data\s+model`),
}

func isArchitecturalContent(content string) bool {
	for _, pattern := range architecturalPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

var debugPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)debug(ging)?`),
	regexp.MustCompile(`(?i)stack\s*trace`),
	regexp.MustCompile(`(?i)core\s*dump`),
	regexp.MustCompile(`(?i)segfault|segmentation`),
	regexp.MustCompile(`(?i)race\s*condition`),
	regexp.MustCompile(`(?i)deadlock`),
	regexp.MustCompile(`(?i)memory\s*leak`),
	regexp.MustCompile(`(?i)panic:`),
}

func isDebuggingScenario(content string) bool {
	for _, pattern := range debugPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

var complexDebugPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)race\s*condition`),
	regexp.MustCompile(`(?i)deadlock`),
	regexp.MustCompile(`(?i)memory\s*leak`),
	regexp.MustCompile(`(?i)concurrency\s*(bug|issue)`),
	regexp.MustCompile(`(?i)intermittent\s*(failure|bug)`),
	regexp.MustCompile(`(?i)heap\s*corruption`),
}

func containsComplexDebugging(content string) bool {
	for _, pattern := range complexDebugPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func isCompilationError(result string) bool {
	lower := strings.ToLower(result)
	return strings.Contains(lower, "compilation failed") ||
		strings.Contains(lower, "build failed") ||
		strings.Contains(lower, "compile error") ||
		strings.Contains(lower, "syntax error") ||
		strings.Contains(lower, "undefined:") ||
		strings.Contains(lower, "cannot find") ||
		strings.Contains(lower, "type mismatch")
}

func isTestError(result string) bool {
	lower := strings.ToLower(result)
	return strings.Contains(lower, "test failed") ||
		strings.Contains(lower, "--- fail") ||
		strings.Contains(lower, "assertion failed") ||
		strings.Contains(lower, "expected:") ||
		strings.Contains(lower, "actual:")
}

func containsModelEscalationRequest(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "need opus") ||
		strings.Contains(lower, "require opus") ||
		strings.Contains(lower, "escalate to opus") ||
		strings.Contains(lower, "switch to opus") ||
		strings.Contains(lower, "need sonnet") ||
		strings.Contains(lower, "require sonnet") ||
		strings.Contains(lower, "escalate to sonnet") ||
		strings.Contains(lower, "need haiku") ||
		strings.Contains(lower, "switch to haiku") ||
		strings.Contains(lower, "this is simple")
}

func extractRequestedModel(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "opus") {
		return ModelOpus
	}
	if strings.Contains(lower, "sonnet") {
		return ModelSonnet
	}
	if strings.Contains(lower, "haiku") || strings.Contains(lower, "this is simple") {
		return ModelHaiku
	}
	return ""
}

func extractContent(msg StreamMessage) string {
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

type Manager struct {
	mu        sync.RWMutex
	selectors map[string]*Selector
	config    Config
}

func NewManager() *Manager {
	return &Manager{
		selectors: make(map[string]*Selector),
		config:    DefaultConfig(),
	}
}

func NewManagerWithConfig(config Config) *Manager {
	return &Manager{
		selectors: make(map[string]*Selector),
		config:    config,
	}
}

func (m *Manager) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *Manager) Register(featureID string, isLeafTask bool, taskCount int) *Selector {
	m.mu.Lock()
	defer m.mu.Unlock()

	selector := NewSelectorWithConfig(featureID, isLeafTask, taskCount, m.config)
	m.selectors[featureID] = selector

	logger.Info("automodel", "Selector registered",
		"featureID", featureID,
		"isLeaf", isLeafTask,
		"taskCount", taskCount,
		"initialModel", selector.currentModel)

	return selector
}

func (m *Manager) Get(featureID string) *Selector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectors[featureID]
}

func (m *Manager) Remove(featureID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.selectors, featureID)
}

func (m *Manager) GetAllSwitches(featureID string) []ModelSwitch {
	m.mu.RLock()
	selector := m.selectors[featureID]
	m.mu.RUnlock()

	if selector == nil {
		return nil
	}
	return selector.Switches()
}

func IsAutoMode(model string) bool {
	return model == "auto"
}
