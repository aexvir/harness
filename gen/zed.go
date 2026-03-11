package gen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	ShowSummary         *bool             `json:"show_summary,omitempty"` // whether to show a summary after the task completes
	ShowCommand         *bool             `json:"show_command,omitempty"` // whether to echo the command in the terminal output
}

type zedConfig struct {
	additionalTasks []ZedTask
}

// ZedOption configures the Zed tasks file generator.
type ZedOption func(*zedConfig)

// WithZedExtraTasks allows manually defining tasks in addition to
// what is discovered from mage targets.
func WithZedExtraTasks(tasks ...ZedTask) ZedOption {
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
//
// User customizations on generated tasks (such as adding "reveal" or "show_summary")
// are preserved across regenerations by matching tasks on their label.
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

		existing, readErr := os.ReadFile(zedTasksFilePath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("failed to read existing tasks file: %w", readErr)
		}
		existingManaged := extractManagedTasks(string(existing))

		merged, err := mergeTaskCustomizations(tasks, existingManaged)
		if err != nil {
			return fmt.Errorf("failed to merge task customizations: %w", err)
		}

		if err := writeZedTasks(merged); err != nil {
			return err
		}

		internal.LogStep(fmt.Sprintf("wrote %d task(s) to %s", len(merged), zedTasksFilePath))
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
func buildManagedSection(tasks []map[string]any) (string, error) {
	var sb strings.Builder

	sb.WriteString("  " + harnessMarkerStart + "\n")

	for i, task := range tasks {
		data, err := marshalTask(task, "  ", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal task: %w", err)
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

// extractManagedTasks parses the managed section from existing file content
// and returns the tasks as raw JSON maps, preserving all fields including
// user customizations.
func extractManagedTasks(content string) []map[string]any {
	lines := strings.Split(content, "\n")
	startIdx, endIdx := findMarkerLines(lines)
	if startIdx < 0 || endIdx <= startIdx {
		return nil
	}

	var jsonLines []string
	for _, line := range lines[startIdx+1 : endIdx] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		jsonLines = append(jsonLines, line)
	}

	jsonContent := "[" + strings.Join(jsonLines, "\n") + "]"

	var tasks []map[string]any
	if err := json.Unmarshal([]byte(jsonContent), &tasks); err != nil {
		return nil
	}

	return tasks
}

// mergeTaskCustomizations merges user customizations from existing tasks
// into newly generated tasks. Tasks are matched by label. Fields present
// in the existing task but absent from the new task are preserved.
func mergeTaskCustomizations(newTasks []ZedTask, existing []map[string]any) ([]map[string]any, error) {
	existingByLabel := make(map[string]map[string]any, len(existing))
	for _, t := range existing {
		if label, ok := t["label"].(string); ok {
			existingByLabel[label] = t
		}
	}

	var result []map[string]any
	for _, task := range newTasks {
		newMap, err := taskToMap(task)
		if err != nil {
			return nil, fmt.Errorf("failed to convert task %q: %w", task.Label, err)
		}

		if existingTask, ok := existingByLabel[task.Label]; ok {
			merged := make(map[string]any, len(existingTask))
			for k, v := range existingTask {
				merged[k] = v
			}
			for k, v := range newMap {
				merged[k] = v
			}
			result = append(result, merged)
		} else {
			result = append(result, newMap)
		}
	}

	return result, nil
}

// taskToMap converts a ZedTask to a map[string]any, respecting omitempty semantics.
func taskToMap(task ZedTask) (map[string]any, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// marshalTask serializes a task map to JSON with label and command fields first
// for readability, followed by remaining fields in alphabetical order.
func marshalTask(task map[string]any, prefix, indent string) ([]byte, error) {
	var otherKeys []string
	for k := range task {
		if k != "label" && k != "command" {
			otherKeys = append(otherKeys, k)
		}
	}
	sort.Strings(otherKeys)

	orderedKeys := make([]string, 0, len(task))
	if _, ok := task["label"]; ok {
		orderedKeys = append(orderedKeys, "label")
	}
	if _, ok := task["command"]; ok {
		orderedKeys = append(orderedKeys, "command")
	}
	orderedKeys = append(orderedKeys, otherKeys...)

	var buf bytes.Buffer
	buf.WriteString("{\n")

	for i, k := range orderedKeys {
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		valJSON, err := json.MarshalIndent(task[k], prefix+indent, indent)
		if err != nil {
			return nil, err
		}

		buf.WriteString(prefix + indent + string(keyJSON) + ": " + string(valJSON))
		if i < len(orderedKeys)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}

	buf.WriteString(prefix + "}")
	return buf.Bytes(), nil
}

// AddZedTask adds or updates a single task in the managed section of .zed/tasks.json.
// If a task with the same label exists in the managed section, it is updated
// while preserving any user customizations. Otherwise, the task is appended.
// This is intended for use by the CLI tool (e.g. via //go:generate directives).
func AddZedTask(task ZedTask) error {
	existing, readErr := os.ReadFile(zedTasksFilePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("failed to read existing tasks file: %w", readErr)
	}
	existingManaged := extractManagedTasks(string(existing))

	newMap, err := taskToMap(task)
	if err != nil {
		return fmt.Errorf("failed to convert task: %w", err)
	}

	found := false
	for i, t := range existingManaged {
		if label, ok := t["label"].(string); ok && label == task.Label {
			merged := make(map[string]any, len(t))
			for k, v := range t {
				merged[k] = v
			}
			for k, v := range newMap {
				merged[k] = v
			}
			existingManaged[i] = merged
			found = true
			break
		}
	}
	if !found {
		existingManaged = append(existingManaged, newMap)
	}

	return writeZedTasks(existingManaged)
}

// writeZedTasks writes the given tasks to the .zed/tasks.json file,
// preserving any user-defined tasks outside the managed section.
func writeZedTasks(tasks []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(zedTasksFilePath), 0o755); err != nil {
		return fmt.Errorf("failed to create .zed directory: %w", err)
	}

	managedSection, err := buildManagedSection(tasks)
	if err != nil {
		return fmt.Errorf("failed to build managed section: %w", err)
	}

	existing, readErr := os.ReadFile(zedTasksFilePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("failed to read existing tasks file: %w", readErr)
	}

	var content string
	if os.IsNotExist(readErr) || len(existing) == 0 {
		content = "[\n" + managedSection + "\n]\n"
	} else {
		content, err = mergeContent(string(existing), managedSection)
		if err != nil {
			return fmt.Errorf("failed to merge tasks: %w", err)
		}
	}

	return os.WriteFile(zedTasksFilePath, []byte(content), 0o644)
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
