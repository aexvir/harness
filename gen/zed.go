package gen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aexvir/harness"
	"github.com/aexvir/harness/internal"
)

const (
	harnessMarkerStart = "// harness:start"
	harnessMarkerEnd   = "// harness:end"
	zedTasksFilePath   = ".zed/tasks.json"
)

// ZedTask represents a single task entry in Zed's tasks.json file.
type ZedTask struct {
	Label               string            `json:"label"`
	Command             string            `json:"command"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	Cwd                 string            `json:"cwd,omitempty"`
	UseNewTerminal      bool              `json:"use_new_terminal,omitempty"`
	AllowConcurrentRuns bool              `json:"allow_concurrent_runs,omitempty"`
	Reveal              string            `json:"reveal,omitempty"`
	Hide                string            `json:"hide,omitempty"`
	Shell               string            `json:"shell,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
}

type zedConfig struct {
	additionalTasks []ZedTask
}

// ZedOption configures the Zed tasks file generator.
type ZedOption func(*zedConfig)

// WithAdditionalTasks allows manually defining tasks in addition to
// what is discovered from mage targets.
func WithAdditionalTasks(tasks ...ZedTask) ZedOption {
	return func(cfg *zedConfig) {
		cfg.additionalTasks = append(cfg.additionalTasks, tasks...)
	}
}

// ZedTasksFile returns a [harness.Task] that generates a .zed/tasks.json file
// by reading mage targets and merging them with any additional tasks provided.
//
// Generated tasks are placed between comment markers (// harness:start / // harness:end)
// to allow coexistence with manually defined tasks in the same file.
// When the file already exists, only the managed section between the markers is updated;
// any tasks outside the markers are left untouched.
func ZedTasksFile(opts ...ZedOption) harness.Task {
	cfg := &zedConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context) error {
		harness.LogStep("generating .zed/tasks.json")

		targets, err := discoverMageTargets(ctx)
		if err != nil {
			return fmt.Errorf("failed to discover mage targets: %w", err)
		}

		var tasks []ZedTask
		for _, t := range targets {
			tasks = append(tasks, ZedTask{
				Label:   fmt.Sprintf("mage %s", t.name),
				Command: fmt.Sprintf("mage %s", t.name),
			})
		}
		tasks = append(tasks, cfg.additionalTasks...)

		managedSection, err := buildManagedSection(tasks)
		if err != nil {
			return fmt.Errorf("failed to build managed section: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(zedTasksFilePath), 0o755); err != nil {
			return fmt.Errorf("failed to create .zed directory: %w", err)
		}

		existing, readErr := os.ReadFile(zedTasksFilePath)

		var content string
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("failed to read existing tasks file: %w", readErr)
		}

		if os.IsNotExist(readErr) || len(existing) == 0 {
			content = "[\n" + managedSection + "\n]\n"
		} else {
			content, err = mergeContent(string(existing), managedSection)
			if err != nil {
				return fmt.Errorf("failed to merge tasks: %w", err)
			}
		}

		if err := os.WriteFile(zedTasksFilePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write tasks file: %w", err)
		}

		internal.LogStep(fmt.Sprintf("wrote %d task(s) to %s", len(tasks), zedTasksFilePath))
		return nil
	}
}

// mageTarget represents a target parsed from mage -l output.
type mageTarget struct {
	name        string
	description string
}

// discoverMageTargets runs mage -l and parses the output.
func discoverMageTargets(ctx context.Context) ([]mageTarget, error) {
	cmd := exec.CommandContext(ctx, "mage", "-l")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mage -l: %w", err)
	}

	return parseMageListOutput(stdout.String()), nil
}

// parseMageListOutput parses the output of mage -l into a list of targets.
func parseMageListOutput(output string) []mageTarget {
	var targets []mageTarget
	lines := strings.Split(output, "\n")

	inTargets := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "Targets:") {
			inTargets = true
			continue
		}

		if !inTargets {
			continue
		}

		// target lines are indented; a non-indented line marks the end of the section
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}

		trimmed := strings.TrimSpace(line)

		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			continue
		}

		target := mageTarget{name: strings.TrimSuffix(parts[0], "*")}
		if len(parts) > 1 {
			target.description = strings.Join(parts[1:], " ")
		}

		targets = append(targets, target)
	}

	return targets
}

// buildManagedSection creates the JSONC content for the managed section
// including the start and end marker comments.
func buildManagedSection(tasks []ZedTask) (string, error) {
	var sb strings.Builder

	sb.WriteString("  " + harnessMarkerStart + "\n")

	for i, task := range tasks {
		data, err := json.MarshalIndent(task, "  ", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal task %q: %w", task.Label, err)
		}

		sb.WriteString("  ")
		sb.Write(data)
		if i < len(tasks)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  " + harnessMarkerEnd)

	return sb.String(), nil
}

// mergeContent merges the managed section into existing file content.
// It removes any previous managed section and inserts the new one
// before the closing bracket of the JSON array.
func mergeContent(existing, managedSection string) (string, error) {
	lines := strings.Split(existing, "\n")

	// remove existing managed section if present
	startIdx, endIdx := findMarkerLines(lines)
	if startIdx >= 0 && endIdx > startIdx {
		lines = append(lines[:startIdx], lines[endIdx+1:]...)
	}

	// find the closing bracket of the array
	closingIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "]" {
			closingIdx = i
			break
		}
	}
	if closingIdx == -1 {
		return "", fmt.Errorf("invalid tasks file: missing closing bracket")
	}

	// if there is JSON content before the closing bracket, ensure a trailing comma
	for i := closingIdx - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || trimmed == "[" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		lines[i] = ensureTrailingComma(lines[i])
		break
	}

	// insert managed section before the closing bracket
	managedLines := strings.Split(managedSection, "\n")

	var result []string
	result = append(result, lines[:closingIdx]...)
	result = append(result, managedLines...)
	result = append(result, lines[closingIdx:]...)

	return strings.Join(result, "\n"), nil
}

// findMarkerLines returns the line indices of the start and end markers.
func findMarkerLines(lines []string) (int, int) {
	start, end := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == harnessMarkerStart {
			start = i
		}
		if trimmed == harnessMarkerEnd {
			end = i
		}
	}
	return start, end
}

// ensureTrailingComma adds a trailing comma to a line if it doesn't already have one.
func ensureTrailingComma(line string) string {
	trimmed := strings.TrimRight(line, " \t")
	if strings.HasSuffix(trimmed, ",") {
		return line
	}
	return trimmed + ","
}
