package gen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aexvir/harness"
)

const (
	defaultSkillsDir = ".claude/skills"

	// mageSkillPrefix is the name prefix used for all generator-owned skills.
	mageSkillPrefix = "mage-"

	skillFileName = "SKILL.md"
)

// AgentSkill represents a single AI agent skill. Skills are written as
// markdown files with YAML frontmatter at <dir>/<name>/SKILL.md, a format
// understood by Claude Code and other agent harnesses.
//
// https://docs.claude.com/en/docs/claude-code/skills
type AgentSkill struct {
	Name        string
	Description string
	// Body is the markdown content written below the frontmatter when the
	// skill file is created for the first time. It is ignored for existing
	// files (user edits are preserved). May be empty.
	Body string
}

// GenerateAgentSkills returns a harness task that ensures every mage target
// has a corresponding skill entry under the skills directory
// (.claude/skills by default).
//
// Merge rules:
//   - Existing file → only the frontmatter description is refreshed; body
//     and any other frontmatter fields are preserved.
//   - Missing file → created with a default body pointing at the mage target.
//   - Stale file (target gone from mage -l) → left as-is; use
//     [Generator.CleanupAgentSkills] to remove.
//
// Manual skills provided via [WithExtraAgentSkills] follow the same rules.
func (g *Generator) GenerateAgentSkills(opts ...AgentSkillsOpt) harness.Task {
	return func(ctx context.Context) error {
		conf := loadAgentSkillsConf(opts)

		targets, err := g.introspectMageTasks(ctx)
		if err != nil {
			return fmt.Errorf("introspect mage tasks: %w", err)
		}

		skills := make([]AgentSkill, 0, len(targets)+len(conf.extra))
		for _, t := range targets {
			skills = append(skills, AgentSkill{
				Name:        mageSkillName(t.name),
				Description: mageSkillDescription(t),
				Body:        mageSkillBody(g.magecmd, t),
			})
		}
		skills = append(skills, conf.extra...)

		return ensureSkillDefinitions(conf.skillsdir, skills)
	}
}

// CleanupAgentSkills returns a harness task that removes stale generator-owned
// skill directories. A directory is stale when its name matches the
// "mage-<target>" pattern but the corresponding target no longer appears in
// mage -l. Non-mage directories and directories for active targets are never
// touched. A generator-owned directory that contains files other than
// SKILL.md is also left intact so any user-added resources survive.
func (g *Generator) CleanupAgentSkills(opts ...AgentSkillsOpt) harness.Task {
	return func(ctx context.Context) error {
		conf := loadAgentSkillsConf(opts)

		targets, err := g.introspectMageTasks(ctx)
		if err != nil {
			return fmt.Errorf("introspect mage tasks: %w", err)
		}

		active := make(map[string]bool, len(targets))
		for _, t := range targets {
			active[mageSkillName(t.name)] = true
		}

		entries, err := os.ReadDir(conf.skillsdir)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read skills dir: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !isMageSkill(name) || active[name] {
				continue
			}
			if err := removeSkillDir(filepath.Join(conf.skillsdir, name)); err != nil {
				return fmt.Errorf("remove skill %q: %w", name, err)
			}
		}

		return nil
	}
}

// UpsertAgentSkills returns a task that upserts skills provided via
// [WithExtraAgentSkills] without running mage -l.
//
// This is intended for use via //go:generate or in any context where mage is
// not available. The same merge semantics as [Generator.GenerateAgentSkills]
// apply.
func (g *Generator) UpsertAgentSkills(opts ...AgentSkillsOpt) harness.Task {
	return func(_ context.Context) error {
		conf := loadAgentSkillsConf(opts)
		return ensureSkillDefinitions(conf.skillsdir, conf.extra)
	}
}

type agentskillsconf struct {
	skillsdir string
	extra     []AgentSkill
}

// AgentSkillsOpt configures the agent skills generator.
type AgentSkillsOpt func(c *agentskillsconf)

// WithAgentSkillsDir overrides the directory that holds skill folders.
// Defaults to ".claude/skills" relative to the working directory.
func WithAgentSkillsDir(path string) AgentSkillsOpt {
	return func(c *agentskillsconf) { c.skillsdir = path }
}

// WithExtraAgentSkills registers additional skills that are not derived from
// mage targets but should still be kept in sync by the generator.
func WithExtraAgentSkills(skills ...AgentSkill) AgentSkillsOpt {
	return func(c *agentskillsconf) { c.extra = append(c.extra, skills...) }
}

func loadAgentSkillsConf(opts []AgentSkillsOpt) *agentskillsconf {
	conf := &agentskillsconf{skillsdir: defaultSkillsDir}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
}

// ensureSkillDefinitions creates or refreshes each skill file. Existing files
// have only their frontmatter description line rewritten; the rest of the
// frontmatter and the markdown body are preserved byte-for-byte.
func ensureSkillDefinitions(dir string, skills []AgentSkill) error {
	for _, skill := range skills {
		if skill.Name == "" {
			return fmt.Errorf("skill name is required")
		}

		path := filepath.Join(dir, skill.Name, skillFileName)
		if err := upsertSkillFile(path, skill); err != nil {
			return fmt.Errorf("upsert skill %q: %w", skill.Name, err)
		}
	}
	return nil
}

// upsertSkillFile writes a new skill file or refreshes the description line
// of an existing one.
func upsertSkillFile(path string, skill AgentSkill) error {
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return writeSkillFile(path, renderSkill(skill))
	}
	if err != nil {
		return err
	}

	updated, err := refreshSkillDescription(string(existing), skill.Description)
	if err != nil {
		return err
	}

	if updated == string(existing) {
		return nil
	}
	return writeSkillFile(path, updated)
}

func writeSkillFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// renderSkill produces the full file content for a brand-new skill file.
func renderSkill(skill AgentSkill) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("name: ")
	sb.WriteString(skill.Name)
	sb.WriteString("\n")
	sb.WriteString("description: ")
	sb.WriteString(skill.Description)
	sb.WriteString("\n")
	sb.WriteString("---\n")

	body := skill.Body
	if body == "" {
		return sb.String()
	}
	if !strings.HasPrefix(body, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

// refreshSkillDescription rewrites the `description:` line inside the leading
// YAML frontmatter block of content. All other frontmatter fields and the
// markdown body are preserved byte-for-byte. If no frontmatter is present,
// one is prepended.
func refreshSkillDescription(content, description string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		// no frontmatter; synthesise one and keep body
		return "---\ndescription: " + description + "\n---\n" + content, nil
	}

	// locate the closing "---" of the frontmatter block
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return "", fmt.Errorf("malformed frontmatter: missing closing ---")
	}

	// replace or insert description within [1, closeIdx)
	descLine := "description: " + description
	found := false
	for i := 1; i < closeIdx; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "description:") {
			lines[i] = descLine
			found = true
			break
		}
	}
	if !found {
		// insert before the closing --- so existing order of fields is preserved
		lines = append(lines[:closeIdx], append([]string{descLine}, lines[closeIdx:]...)...)
	}

	return strings.Join(lines, "\n"), nil
}

// removeSkillDir removes a generator-owned skill directory. For safety the
// directory is only removed when it contains nothing besides SKILL.md, so any
// user-added resources (extra markdown, scripts, assets) survive cleanup.
func removeSkillDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != skillFileName {
			return fmt.Errorf("%s contains user files; refusing to remove", path)
		}
	}
	return os.RemoveAll(path)
}

func mageSkillName(target string) string {
	return mageSkillPrefix + target
}

func isMageSkill(name string) bool {
	return strings.HasPrefix(name, mageSkillPrefix)
}

func mageSkillDescription(t target) string {
	if t.description == "" {
		return fmt.Sprintf("Runs the `mage %s` target.", t.name)
	}
	return fmt.Sprintf("%s. Invokes `mage %s`.", t.description, t.name)
}

func mageSkillBody(magecmd string, t target) string {
	summary := t.description
	if summary == "" {
		summary = fmt.Sprintf("Executes the `%s` mage target.", t.name)
	}
	return fmt.Sprintf(
		"# mage %s\n\n%s\n\n## Usage\n\nRun the task from the project root:\n\n```\n%s %s\n```\n",
		t.name, summary, magecmd, t.name,
	)
}
