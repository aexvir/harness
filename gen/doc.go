// Package gen provides code generators that automate the creation and maintenance
// of editor and tooling configuration files based on the mage targets defined in
// a project.
//
// Generators read the available mage targets via [introspectMageTasks] and use
// that information to produce or update configuration files so that all targets
// are always reachable from within the editor or tooling without manual upkeep.
//
// Generators are designed to be idempotent and non-destructive: they only
// manage the entries they own, leaving any user-added configuration untouched.
// Per-entry ownership is determined by a naming convention rather than file-level
// markers, so users can freely edit any other property of a generated entry and
// those changes will survive subsequent generator runs.
//
// Create a [Generator] with [New] and call its methods:
//   - [Generator.GenerateZedTasks] / [Generator.CleanupZedTasks] – Zed editor task integration
package gen
