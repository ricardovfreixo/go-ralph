package context

import (
	"strings"
)

const (
	// Approximate tokens per character (conservative estimate for English)
	TokensPerChar = 0.25

	// Section markers for context structure
	SectionMarkerProject = "# Project Context"
	SectionMarkerProgress = "# Progress from Previous Features"
	SectionMarkerFeature = "# Current Feature:"
	SectionMarkerParent = "## Context from Parent"
)

// EstimateTokens estimates token count from text length
// Uses conservative ratio: ~4 characters per token for English
func EstimateTokens(text string) int64 {
	return int64(float64(len(text)) * TokensPerChar)
}

// TruncateToTokens truncates text to approximately fit within token budget
// Preserves structure by prioritizing recent content
func TruncateToTokens(text string, maxTokens int64) string {
	if maxTokens <= 0 {
		return text
	}

	currentTokens := EstimateTokens(text)
	if currentTokens <= maxTokens {
		return text
	}

	// Calculate target character count
	targetChars := int(float64(maxTokens) / TokensPerChar)

	// Preserve structure by keeping start and end
	if len(text) <= targetChars {
		return text
	}

	// Keep header (first ~20%) and tail (last ~80%)
	headerSize := targetChars / 5
	tailSize := targetChars - headerSize - 50 // Leave room for truncation marker

	if headerSize+tailSize >= len(text) {
		return text
	}

	header := text[:headerSize]
	tail := text[len(text)-tailSize:]

	return header + "\n\n[... context truncated to fit budget ...]\n\n" + tail
}

// ExtractEssentialContext extracts the most important parts of context for a child
// Prioritizes: current feature, project context, recent progress
func ExtractEssentialContext(fullContext string, maxTokens int64) string {
	if maxTokens <= 0 {
		maxTokens = DefaultMinBudget
	}

	sections := parseContextSections(fullContext)

	var result strings.Builder
	remainingTokens := maxTokens

	// Priority 1: Current feature context (most important for child)
	if feature, ok := sections["feature"]; ok && len(feature) > 0 {
		featureTokens := EstimateTokens(feature)
		if featureTokens <= remainingTokens/2 {
			result.WriteString(feature)
			result.WriteString("\n\n")
			remainingTokens -= featureTokens
		} else {
			// Truncate feature context if too large
			truncated := TruncateToTokens(feature, remainingTokens/2)
			result.WriteString(truncated)
			result.WriteString("\n\n")
			remainingTokens -= EstimateTokens(truncated)
		}
	}

	// Priority 2: Project context summary
	if project, ok := sections["project"]; ok && len(project) > 0 {
		projectTokens := EstimateTokens(project)
		if projectTokens <= remainingTokens/2 {
			result.WriteString(SectionMarkerProject)
			result.WriteString("\n")
			result.WriteString(project)
			result.WriteString("\n\n")
			remainingTokens -= projectTokens
		} else if remainingTokens > DefaultMinBudget/2 {
			// Summarize project context if too large
			truncated := TruncateToTokens(project, remainingTokens/3)
			result.WriteString(SectionMarkerProject)
			result.WriteString("\n[Summarized]\n")
			result.WriteString(truncated)
			result.WriteString("\n\n")
			remainingTokens -= EstimateTokens(truncated)
		}
	}

	// Priority 3: Recent progress (only if budget allows)
	if progress, ok := sections["progress"]; ok && len(progress) > 0 && remainingTokens > DefaultMinBudget/4 {
		progressTokens := EstimateTokens(progress)
		if progressTokens <= remainingTokens {
			result.WriteString(SectionMarkerProgress)
			result.WriteString("\n")
			result.WriteString(progress)
		} else {
			// Extract only recent progress entries
			recent := extractRecentProgress(progress, remainingTokens)
			if len(recent) > 0 {
				result.WriteString(SectionMarkerProgress)
				result.WriteString("\n[Recent entries only]\n")
				result.WriteString(recent)
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// parseContextSections splits context into logical sections
func parseContextSections(context string) map[string]string {
	sections := make(map[string]string)

	lines := strings.Split(context, "\n")
	var currentSection string
	var sectionContent strings.Builder

	flushSection := func() {
		if currentSection != "" {
			sections[currentSection] = strings.TrimSpace(sectionContent.String())
			sectionContent.Reset()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section markers
		if strings.HasPrefix(trimmed, SectionMarkerProject) {
			flushSection()
			currentSection = "project"
			continue
		}
		if strings.HasPrefix(trimmed, SectionMarkerProgress) {
			flushSection()
			currentSection = "progress"
			continue
		}
		if strings.HasPrefix(trimmed, SectionMarkerFeature) || strings.HasPrefix(trimmed, "# Current Feature:") {
			flushSection()
			currentSection = "feature"
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
			continue
		}
		if strings.HasPrefix(trimmed, SectionMarkerParent) {
			flushSection()
			currentSection = "parent"
			continue
		}

		// Accumulate content for current section
		if currentSection != "" {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	flushSection()
	return sections
}

// extractRecentProgress extracts the most recent progress entries within token budget
func extractRecentProgress(progress string, maxTokens int64) string {
	// Split by feature headers (## Feature X)
	entries := splitProgressEntries(progress)

	if len(entries) == 0 {
		return TruncateToTokens(progress, maxTokens)
	}

	// Take entries from the end (most recent) until budget exhausted
	var result []string
	usedTokens := int64(0)

	for i := len(entries) - 1; i >= 0; i-- {
		entryTokens := EstimateTokens(entries[i])
		if usedTokens+entryTokens > maxTokens {
			break
		}
		result = append([]string{entries[i]}, result...)
		usedTokens += entryTokens
	}

	return strings.Join(result, "\n\n")
}

// splitProgressEntries splits progress into individual feature entries
func splitProgressEntries(progress string) []string {
	var entries []string
	var current strings.Builder
	lines := strings.Split(progress, "\n")

	for _, line := range lines {
		// New entry starts with ## Feature header
		if strings.HasPrefix(line, "## Feature") || strings.HasPrefix(line, "## ") {
			if current.Len() > 0 {
				entries = append(entries, strings.TrimSpace(current.String()))
				current.Reset()
			}
		}
		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		entries = append(entries, strings.TrimSpace(current.String()))
	}

	return entries
}

// SummarizeContext creates a condensed version of context
func SummarizeContext(context string, maxTokens int64) string {
	if maxTokens <= 0 {
		maxTokens = DefaultMinBudget
	}

	currentTokens := EstimateTokens(context)
	if currentTokens <= maxTokens {
		return context
	}

	// Extract essential parts
	return ExtractEssentialContext(context, maxTokens)
}

// PrepareChildContext prepares context for a child feature
// Takes parent's context and budget, returns appropriately sized child context
func PrepareChildContext(parentContext string, childBudget int64, featureTitle string, featureTasks []string) string {
	var result strings.Builder

	// Add child feature header
	result.WriteString("# Sub-Feature: ")
	result.WriteString(featureTitle)
	result.WriteString("\n\n")

	// Add tasks if provided
	if len(featureTasks) > 0 {
		result.WriteString("## Tasks\n")
		for _, task := range featureTasks {
			result.WriteString("- [ ] ")
			result.WriteString(task)
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	// Calculate how much budget remains for parent context
	headerTokens := EstimateTokens(result.String())
	contextBudget := childBudget - headerTokens - 1000 // Reserve 1000 tokens for working room

	if contextBudget <= 0 {
		return result.String()
	}

	// Extract and summarize parent context
	if parentContext != "" {
		essential := ExtractEssentialContext(parentContext, contextBudget)
		if len(essential) > 0 {
			result.WriteString("## Context from Parent\n")
			result.WriteString(essential)
			result.WriteString("\n")
		}
	}

	return result.String()
}
