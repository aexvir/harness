package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aexvir/harness"
)

// ZedTask represents a single task in Zed's tasks.json format
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
	Shell               interface{}       `json:"shell,omitempty"`
	ShowSummary         bool              `json:"show_summary,omitempty"`
	ShowOutput          bool              `json:"show_output,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
}

// ZedTasksConfig holds the configuration for Zed task generation
type ZedTasksConfig struct {
	outputPath   string
	extraTasks   []ZedTask
	taskPrefix   string
	generatedTag string
}

// ZedTasksOpt is a function that modifies ZedTasksConfig
type ZedTasksOpt func(*ZedTasksConfig)

// WithZedOutputPath sets the output path for the tasks.json file
func WithZedOutputPath(path string) ZedTasksOpt {
	return func(c *ZedTasksConfig) {
		c.outputPath = path
	}
}

// WithZedExtraTasks adds manual tasks to the generated file
func WithZedExtraTasks(tasks ...ZedTask) ZedTasksOpt {
	return func(c *ZedTasksConfig) {
		c.extraTasks = append(c.extraTasks, tasks...)
	}
}

// WithZedTaskPrefix sets a prefix for generated task labels
func WithZedTaskPrefix(prefix string) ZedTasksOpt {
	return func(c *ZedTasksConfig) {
		c.taskPrefix = prefix
	}
}

// ZedTasks generates a .zed/tasks.json file from mage targets
func ZedTasks(opts ...ZedTasksOpt) harness.Task {
	config := ZedTasksConfig{
		outputPath:   ".zed/tasks.json",
		taskPrefix:   "mage: ",
		generatedTag: "harness",
	}

	for _, opt := range opts {
		opt(&config)
	}

	return func(ctx context.Context) error {
		harness.LogStep("Generating Zed tasks from mage targets")

		// Get mage targets
		targets, err := getMageTargets("mage")
		if err != nil {
			return fmt.Errorf("failed to get mage targets: %w", err)
		}

		// Create tasks from mage targets
		var generatedTasks []ZedTask
		for _, target := range targets {
			task := ZedTask{
				Label:       config.taskPrefix + target.Name,
				Command:     "mage",
				Args:        []string{target.Name},
				Reveal:      "always",
				ShowSummary: true,
				ShowOutput:  true,
				Tags:        []string{config.generatedTag},
			}
			generatedTasks = append(generatedTasks, task)
		}

		// Add extra tasks
		for _, task := range config.extraTasks {
			if task.Tags == nil {
				task.Tags = []string{}
			}
			task.Tags = append(task.Tags, config.generatedTag)
			generatedTasks = append(generatedTasks, task)
		}

		// Merge with existing tasks and write file
		return writeZedTasks(config.outputPath, generatedTasks, config.generatedTag)
	}
}

// MageTarget represents a parsed mage target
type MageTarget struct {
	Name        string
	Description string
}

// getMageTargets parses mage -l output to extract targets
func getMageTargets(mageCmd string) ([]MageTarget, error) {
	cmd := exec.Command(mageCmd, "-l")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run %s -l: %w", mageCmd, err)
	}

	var targets []MageTarget
	lines := strings.Split(string(output), "\n")

	// Parse mage -l output format:
	// Targets:
	//   targetName    description
	inTargetsSection := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "Targets:" {
			inTargetsSection = true
			continue
		}

		if inTargetsSection && line != "" {
			// Split on whitespace, first part is target name, rest is description
			parts := strings.Fields(line)
			if len(parts) > 0 {
				target := MageTarget{
					Name: parts[0],
				}
				if len(parts) > 1 {
					target.Description = strings.Join(parts[1:], " ")
				}
				targets = append(targets, target)
			}
		}
	}

	return targets, nil
}

// writeZedTasks writes tasks to the Zed tasks.json file, merging with existing content
func writeZedTasks(outputPath string, generatedTasks []ZedTask, generatedTag string) error {
	// Ensure the directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var existingTasks []ZedTask

	// Read existing file if it exists
	if data, err := os.ReadFile(outputPath); err == nil {
		if err := json.Unmarshal(data, &existingTasks); err != nil {
			return fmt.Errorf("failed to parse existing tasks file: %w", err)
		}
	}

	// Filter out previously generated tasks
	var filteredTasks []ZedTask
	for _, task := range existingTasks {
		isGenerated := false
		for _, tag := range task.Tags {
			if tag == generatedTag {
				isGenerated = true
				break
			}
		}
		if !isGenerated {
			filteredTasks = append(filteredTasks, task)
		}
	}

	// Combine filtered existing tasks with new generated tasks
	allTasks := append(filteredTasks, generatedTasks...)

	// Write the file
	data, err := json.MarshalIndent(allTasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	harness.LogStep(fmt.Sprintf("Generated %d tasks to %s", len(generatedTasks), outputPath))
	return nil
}
