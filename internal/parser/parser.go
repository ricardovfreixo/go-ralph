package parser

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type PRD struct {
	Title      string
	Context    string
	Features   []Feature
	RawContent string
}

type Feature struct {
	ID                 string
	Title              string
	Description        string
	ExecutionMode      string // "sequential" or "parallel"
	Model              string // "sonnet", "opus", "haiku"
	Tasks              []Task
	AcceptanceCriteria []string
	RawContent         string
}

type Task struct {
	ID          string
	Description string
	Completed   bool
}

var (
	h1Regex       = regexp.MustCompile(`^#\s+(.+)$`)
	h2Regex       = regexp.MustCompile(`^##\s+(.+)$`)
	taskRegex     = regexp.MustCompile(`^[-*]\s+\[([ xX])\]\s+(.+)$`)
	metaRegex     = regexp.MustCompile(`(?i)^(execution|mode|model|run):\s*(.+)$`)
	criteriaRegex = regexp.MustCompile(`(?i)^(acceptance|criteria|test):\s*(.+)$`)
)

func ParsePRD(path string) (*PRD, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRD file: %w", err)
	}

	return ParsePRDContent(string(content))
}

func ParsePRDContent(content string) (*PRD, error) {
	prd := &PRD{
		RawContent: content,
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentFeature *Feature
	var currentSection string
	var descriptionLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if matches := h1Regex.FindStringSubmatch(line); matches != nil {
			prd.Title = matches[1]
			currentSection = "context"
			continue
		}

		if matches := h2Regex.FindStringSubmatch(line); matches != nil {
			if currentFeature != nil {
				currentFeature.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
				prd.Features = append(prd.Features, *currentFeature)
			}

			currentFeature = &Feature{
				Title:         matches[1],
				ID:            generateID(matches[1]),
				ExecutionMode: "sequential",
				Model:         "sonnet",
			}
			currentSection = "feature"
			descriptionLines = nil
			continue
		}

		if currentSection == "context" && prd.Title != "" {
			prd.Context += line + "\n"
			continue
		}

		if currentFeature == nil {
			continue
		}

		if matches := taskRegex.FindStringSubmatch(line); matches != nil {
			task := Task{
				ID:          generateID(matches[2]),
				Description: matches[2],
				Completed:   matches[1] != " ",
			}
			currentFeature.Tasks = append(currentFeature.Tasks, task)
			continue
		}

		if matches := metaRegex.FindStringSubmatch(line); matches != nil {
			value := strings.ToLower(strings.TrimSpace(matches[2]))
			switch strings.ToLower(matches[1]) {
			case "execution", "mode", "run":
				if value == "parallel" || value == "concurrent" {
					currentFeature.ExecutionMode = "parallel"
				} else {
					currentFeature.ExecutionMode = "sequential"
				}
			case "model":
				if value == "opus" || value == "haiku" || value == "sonnet" {
					currentFeature.Model = value
				}
			}
			continue
		}

		if matches := criteriaRegex.FindStringSubmatch(line); matches != nil {
			currentFeature.AcceptanceCriteria = append(currentFeature.AcceptanceCriteria, matches[2])
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "- ") && !strings.Contains(line, "[ ]") && !strings.Contains(line, "[x]") {
			trimmed := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if strings.HasPrefix(strings.ToLower(trimmed), "acceptance:") || strings.HasPrefix(strings.ToLower(trimmed), "criteria:") {
				currentFeature.AcceptanceCriteria = append(currentFeature.AcceptanceCriteria, strings.TrimPrefix(trimmed, strings.Split(trimmed, ":")[0]+":"))
			} else {
				descriptionLines = append(descriptionLines, line)
			}
			continue
		}

		descriptionLines = append(descriptionLines, line)
	}

	if currentFeature != nil {
		currentFeature.Description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
		prd.Features = append(prd.Features, *currentFeature)
	}

	prd.Context = strings.TrimSpace(prd.Context)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning PRD: %w", err)
	}

	return prd, nil
}

func generateID(title string) string {
	hash := sha256.Sum256([]byte(title))
	return fmt.Sprintf("%x", hash[:8])
}

func (f *Feature) ToPrompt(context string) string {
	var sb strings.Builder

	sb.WriteString("# Project Context\n\n")
	sb.WriteString(context)
	sb.WriteString("\n\n")

	sb.WriteString("# Current Feature: ")
	sb.WriteString(f.Title)
	sb.WriteString("\n\n")

	sb.WriteString(f.Description)
	sb.WriteString("\n\n")

	if len(f.Tasks) > 0 {
		sb.WriteString("## Tasks\n\n")
		for _, task := range f.Tasks {
			checkbox := "[ ]"
			if task.Completed {
				checkbox = "[x]"
			}
			sb.WriteString(fmt.Sprintf("- %s %s\n", checkbox, task.Description))
		}
		sb.WriteString("\n")
	}

	if len(f.AcceptanceCriteria) > 0 {
		sb.WriteString("## Acceptance Criteria\n\n")
		for _, criteria := range f.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- %s\n", criteria))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Implement all tasks listed above\n")
	sb.WriteString("2. Write tests for each implemented feature\n")
	sb.WriteString("3. Run all tests and ensure they pass\n")
	sb.WriteString("4. Verify all acceptance criteria are met\n")
	sb.WriteString("5. Update progress.md with your changes\n")

	return sb.String()
}
