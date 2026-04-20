package gen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertAgentSkills(t *testing.T) {
	t.Run(
		"creates directory and SKILL.md when missing",
		func(t *testing.T) {
			dir := t.TempDir()

			require.NoError(t, New().UpsertAgentSkills(
				WithAgentSkillsDir(dir),
				WithExtraAgentSkills(
					AgentSkill{Name: "alpha", Description: "first", Body: "# alpha\n\nbody\n"},
					AgentSkill{Name: "beta", Description: "second"},
				),
			)(t.Context()))

			alpha, err := os.ReadFile(filepath.Join(dir, "alpha", "SKILL.md"))
			require.NoError(t, err)
			assert.Contains(t, string(alpha), "name: alpha")
			assert.Contains(t, string(alpha), "description: first")
			assert.Contains(t, string(alpha), "# alpha")

			beta, err := os.ReadFile(filepath.Join(dir, "beta", "SKILL.md"))
			require.NoError(t, err)
			assert.Contains(t, string(beta), "name: beta")
			assert.Contains(t, string(beta), "description: second")
		},
	)

	t.Run(
		"re-run refreshes description; preserves body and unknown frontmatter fields",
		func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "mine", "SKILL.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

			seed := "---\n" +
				"name: mine\n" +
				"description: old\n" +
				"color: blue\n" +
				"---\n" +
				"\n" +
				"# custom body\n" +
				"\n" +
				"user-written content\n"
			require.NoError(t, os.WriteFile(path, []byte(seed), 0o644))

			require.NoError(t, New().UpsertAgentSkills(
				WithAgentSkillsDir(dir),
				WithExtraAgentSkills(AgentSkill{
					Name:        "mine",
					Description: "new",
					Body:        "should NOT appear\n",
				}),
			)(t.Context()))

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			out := string(data)

			assert.Contains(t, out, "description: new", "description must be refreshed")
			assert.Contains(t, out, "color: blue", "unknown frontmatter fields must survive")
			assert.Contains(t, out, "user-written content", "body must be preserved")
			assert.NotContains(t, out, "should NOT appear", "default body ignored on existing file")
		},
	)

	t.Run(
		"missing name returns error",
		func(t *testing.T) {
			err := New().UpsertAgentSkills(
				WithAgentSkillsDir(t.TempDir()),
				WithExtraAgentSkills(AgentSkill{Description: "x"}),
			)(t.Context())
			require.Error(t, err)
		},
	)
}

func TestGenerateAgentSkills(t *testing.T) {
	skipOnWindows(t)

	t.Run(
		"generates a skill directory per mage target",
		func(t *testing.T) {
			dir := t.TempDir()

			require.NoError(t, New(
				WithMageCommand("testdata/fakemage.sh"),
			).GenerateAgentSkills(
				WithAgentSkillsDir(dir),
				WithExtraAgentSkills(AgentSkill{Name: "extra", Description: "manual"}),
			)(t.Context()))

			for _, target := range []string{"build", "format", "lint", "test"} {
				path := filepath.Join(dir, "mage-"+target, "SKILL.md")
				data, err := os.ReadFile(path)
				require.NoErrorf(t, err, "skill file for %s must exist", target)
				assert.Contains(t, string(data), "name: mage-"+target)
			}

			extra, err := os.ReadFile(filepath.Join(dir, "extra", "SKILL.md"))
			require.NoError(t, err)
			assert.Contains(t, string(extra), "name: extra")
		},
	)

	t.Run(
		"returns error when mage command is not found",
		func(t *testing.T) {
			err := New(
				WithMageCommand("/nonexistent/mage"),
			).GenerateAgentSkills(
				WithAgentSkillsDir(t.TempDir()),
			)(t.Context())
			require.Error(t, err)
		},
	)
}

func TestCleanupAgentSkills(t *testing.T) {
	skipOnWindows(t)

	t.Run(
		"removes stale mage skills; keeps active and non-mage skills",
		func(t *testing.T) {
			dir := t.TempDir()

			seedSkill(t, dir, "mage-build", "---\nname: mage-build\n---\nbody\n")
			seedSkill(t, dir, "mage-stale", "---\nname: mage-stale\n---\nbody\n")
			seedSkill(t, dir, "custom", "---\nname: custom\n---\nbody\n")

			require.NoError(t, New(
				WithMageCommand("testdata/fakemage.sh"),
			).CleanupAgentSkills(
				WithAgentSkillsDir(dir),
			)(t.Context()))

			assert.DirExists(t, filepath.Join(dir, "mage-build"), "active skill kept")
			assert.DirExists(t, filepath.Join(dir, "custom"), "non-mage skill kept")

			_, err := os.Stat(filepath.Join(dir, "mage-stale"))
			assert.True(t, os.IsNotExist(err), "stale skill removed")
		},
	)

	t.Run(
		"refuses to remove skill dirs with user-added files",
		func(t *testing.T) {
			dir := t.TempDir()
			seedSkill(t, dir, "mage-stale", "---\nname: mage-stale\n---\nbody\n")
			require.NoError(t, os.WriteFile(
				filepath.Join(dir, "mage-stale", "notes.md"),
				[]byte("user notes"), 0o644,
			))

			err := New(
				WithMageCommand("testdata/fakemage.sh"),
			).CleanupAgentSkills(WithAgentSkillsDir(dir))(t.Context())
			require.Error(t, err)

			assert.DirExists(t, filepath.Join(dir, "mage-stale"), "user files preserved")
		},
	)

	t.Run(
		"no-op when skills dir does not exist",
		func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "missing")
			require.NoError(t, New(
				WithMageCommand("testdata/fakemage.sh"),
			).CleanupAgentSkills(WithAgentSkillsDir(dir))(t.Context()))
		},
	)
}

func seedSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name, "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
