package gen

import (
	"context"
	"fmt"
	"strings"

	"github.com/aexvir/harness"
	"github.com/tailscale/hujson"
)

const (
	defaultTasksFile = ".zed/tasks.json"

	// mageTaskPrefix is the label prefix used for all generator-owned task entries.
	mageTaskPrefix = "mage: "
)

// ZedTask represents a single entry in Zed's tasks.json file.
//
// https://zed.dev/docs/tasks
type ZedTask struct {
	Label               string            `json:"label"`
	Command             string            `json:"command"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	CWD                 string            `json:"cwd,omitempty"`
	UseNewTerminal      *bool             `json:"use_new_terminal,omitempty"`
	AllowConcurrentRuns *bool             `json:"allow_concurrent_runs,omitempty"`
	Reveal              string            `json:"reveal,omitempty"`
	Hide                string            `json:"hide,omitempty"`
	ShowSummary         *bool             `json:"show_summary,omitempty"`
	ShowCommand         *bool             `json:"show_command,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
}

// GenerateZedTasks returns a harness task that ensures every mage target has a
// corresponding entry in the Zed tasks file (.zed/tasks.json by default).
//
// Merge rules:
//   - Existing entry → only command and args are refreshed; all other fields preserved.
//   - Missing entry → appended.
//   - Stale entry (target gone from mage -l) → left as-is; use [Generator.CleanupZedTasks] to remove.
//
// Manual tasks provided via [WithExtraTasks] follow the same rules.
func (g *Generator) GenerateZedTasks(opts ...ZedTasksOpt) harness.Task {
	return func(ctx context.Context) error {
		conf := loadZedConf(opts)

		targets, err := g.introspectMageTasks(ctx)
		if err != nil {
			return fmt.Errorf("introspect mage tasks: %w", err)
		}

		tasks := make([]ZedTask, 0, len(targets)+len(conf.extra))
		for _, t := range targets {
			tasks = append(tasks, ZedTask{Label: mageTaskLabel(t.name), Command: g.magecmd, Args: []string{t.name}})
		}
		tasks = append(tasks, conf.extra...)

		return ensureTaskDefinitions(conf.taskfile, tasks)
	}
}

// CleanupZedTasks returns a harness task that removes stale generator-owned
// entries from the Zed tasks file. An entry is stale when its label matches
// the "mage: <target>" pattern but the corresponding target no longer appears
// in mage -l. Non-mage entries and entries for active targets are never touched.
func (g *Generator) CleanupZedTasks(opts ...ZedTasksOpt) harness.Task {
	return func(ctx context.Context) error {
		conf := loadZedConf(opts)

		targets, err := g.introspectMageTasks(ctx)
		if err != nil {
			return fmt.Errorf("introspect mage tasks: %w", err)
		}

		active := make(map[string]bool, len(targets))
		for _, t := range targets {
			active[mageTaskLabel(t.name)] = true
		}

		def, err := loadJsonArrayFile(conf.taskfile)
		if err != nil {
			return fmt.Errorf("read tasks file: %w", err)
		}

		arr := def.getRootArray()
		if arr == nil {
			return fmt.Errorf("tasks file is not an array")
		}

		kept := arr.Elements[:0]
		for _, elem := range arr.Elements {
			label := def.readObjectMemberValue(def.elemAsObject(elem), "label")
			if isMageTask(label) && !active[label] {
				continue
			}
			kept = append(kept, elem)
		}
		arr.Elements = kept

		return def.save(conf.taskfile)
	}
}

// UpsertZedTasks returns a task that upserts tasks provided via [WithExtraTasks]
// into the Zed tasks file without running mage -l.
//
// This is intended for use via //go:generate or in any context where mage is
// not available. The same merge semantics as [Generator.GenerateZedTasks] apply.
func (g *Generator) UpsertZedTasks(opts ...ZedTasksOpt) harness.Task {
	return func(_ context.Context) error {
		conf := loadZedConf(opts)
		return ensureTaskDefinitions(conf.taskfile, conf.extra)
	}
}

type zedtasksconf struct {
	taskfile string
	extra    []ZedTask
}

// ZedTasksOpt configures the Zed tasks generator.
type ZedTasksOpt func(c *zedtasksconf)

// WithZedTasksFile overrides the path to the Zed tasks file.
// Defaults to ".zed/tasks.json" relative to the working directory.
func WithZedTasksFile(path string) ZedTasksOpt {
	return func(c *zedtasksconf) { c.taskfile = path }
}

// WithExtraTasks registers additional tasks that are not derived from mage
// targets but should still be kept in sync by the generator.
func WithExtraTasks(tasks ...ZedTask) ZedTasksOpt {
	return func(c *zedtasksconf) { c.extra = append(c.extra, tasks...) }
}

// ensureTaskDefinitions loads the array from path, upserts tasks, and saves it back.
// For each task: if an entry with the same label exists, only its command and
// args members are updated; all other members are left untouched. Missing
// entries are appended.
func ensureTaskDefinitions(path string, tasks []ZedTask) error {
	def, err := loadJsonArrayFile(path)
	if err != nil {
		return fmt.Errorf("read tasks file: %w", err)
	}

	arr := def.getRootArray()
	if arr == nil {
		return fmt.Errorf("tasks file is not an array")
	}

	for _, task := range tasks {
		idx := def.findArrayElementByLabel(arr, task.Label)

		if idx < 0 { // task with specified label not found, create it and continue
			if err := def.appendArrayElement(arr, task); err != nil {
				return fmt.Errorf("append task %q: %w", task.Label, err)
			}
			continue
		}

		// task exists, ensure it's an object and overwrite command and args
		obj, ok := arr.Elements[idx].Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf("task with label %q is not a valid json object; got %v", task.Label, arr.Elements[idx].Value)
		}
		if err := def.setObjectMember(obj, "command", task.Command); err != nil {
			return fmt.Errorf("set command for %q: %w", task.Label, err)
		}
		if err := def.setObjectMember(obj, "args", task.Args); err != nil {
			return fmt.Errorf("set args for %q: %w", task.Label, err)
		}
	}

	return def.save(path)
}

func mageTaskLabel(name string) string {
	return mageTaskPrefix + name
}

func isMageTask(label string) bool {
	return strings.HasPrefix(label, mageTaskPrefix)
}

func loadZedConf(opts []ZedTasksOpt) *zedtasksconf {
	conf := &zedtasksconf{
		taskfile: defaultTasksFile,
	}

	for _, opt := range opts {
		opt(conf)
	}

	return conf
}
