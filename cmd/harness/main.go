// harness is a CLI tool for generating editor configuration files.
//
// Usage:
//
//	harness zed task -label="..." -command="..." [flags]
//
// The tool is designed to work with //go:generate directives so that projects
// not using magefiles can still generate Zed tasks:
//
//	//go:generate harness zed task -label="build" -command="go build ./..."
//	//go:generate harness zed task -label="test" -command="go test ./..." -show-summary
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aexvir/harness/gen"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "zed":
		handleZed(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleZed(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: harness zed <subcommand>\n\nsubcommands:\n  task    add or update a task in .zed/tasks.json\n")
		os.Exit(1)
	}

	switch args[0] {
	case "task":
		handleZedTask(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown zed subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func handleZedTask(args []string) {
	fs := flag.NewFlagSet("harness zed task", flag.ExitOnError)

	label := fs.String("label", "", "task label (required)")
	command := fs.String("command", "", "task command (required)")
	reveal := fs.String("reveal", "", "reveal behavior (always, never)")
	hide := fs.String("hide", "", "hide behavior (on_success, never)")
	cwd := fs.String("cwd", "", "working directory for the task")
	shell := fs.String("shell", "", "shell to use")
	tags := fs.String("tags", "", "comma-separated list of tags")
	showSummary := fs.Bool("show-summary", false, "show task summary")
	showCommand := fs.Bool("show-command", false, "show command in output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: harness zed task -label=\"...\" -command=\"...\" [flags]\n\nflags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *label == "" || *command == "" {
		fmt.Fprintf(os.Stderr, "error: -label and -command are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	task := gen.ZedTask{
		Label:   *label,
		Command: *command,
	}

	// only set optional fields when explicitly provided
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "reveal":
			task.Reveal = *reveal
		case "hide":
			task.Hide = *hide
		case "cwd":
			task.Cwd = *cwd
		case "shell":
			task.Shell = *shell
		case "tags":
			if *tags != "" {
				task.Tags = strings.Split(*tags, ",")
			}
		case "show-summary":
			v := *showSummary
			task.ShowSummary = &v
		case "show-command":
			v := *showCommand
			task.ShowCommand = &v
		}
	})

	if err := gen.AddZedTask(task); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `usage: harness <command> [arguments]

commands:
  zed    generate Zed editor configuration files

examples:
  harness zed task -label="build" -command="go build ./..."
  harness zed task -label="test" -command="go test ./..." -show-summary
`)
}
