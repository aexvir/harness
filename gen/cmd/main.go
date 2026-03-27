// Command gen is a standalone CLI for the harness code generators.
// It can be used via //go:generate without requiring mage or the harness
// runtime to be present in the consuming project.
//
// Usage:
//
//	go run github.com/aexvir/harness/gen/cmd <generator> <action> [flags]
//
// Example:
//
//	//go:generate go run github.com/aexvir/harness/gen/cmd zed task --label="test" --command="go test ./..."
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aexvir/harness/gen"
	"github.com/urfave/cli/v3"
)

func main() {
	if err := buildApp().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func buildApp() *cli.Command {
	return &cli.Command{
		Name:  "gen",
		Usage: "harness code generators",
		// called when no subcommand matches or no args given
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() > 0 {
				return fmt.Errorf("unknown generator %q; supported: zed", cmd.Args().First())
			}
			return fmt.Errorf("expected a generator command")
		},
		Commands: []*cli.Command{
			{
				Name:  "zed",
				Usage: "Zed editor generators",
				// called when no subcommand of zed matches
				Action: func(_ context.Context, cmd *cli.Command) error {
					if cmd.NArg() > 0 {
						return fmt.Errorf("unknown zed action %q; supported: task", cmd.Args().First())
					}
					return fmt.Errorf("expected an action for zed")
				},
				Commands: []*cli.Command{
					{
						Name:  "task",
						Usage: "upsert a task into .zed/tasks.json",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "label",
								Usage:    "task `label`",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "command",
								Usage:    "`command` to run",
								Required: true,
							},
							&cli.StringSliceFlag{
								Name:  "args",
								Usage: "`argument` passed to command (may be repeated)",
							},
							&cli.StringFlag{
								Name:  "file",
								Usage: "path to tasks.json `file`",
								Value: ".zed/tasks.json",
							},
							&cli.StringFlag{
								Name:  "reveal",
								Usage: "when to reveal the task panel: always, never, focus",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							args := cmd.StringSlice("args")
							if len(args) == 0 {
								args = nil
							}
							task := gen.ZedTask{
								Label:   cmd.String("label"),
								Command: cmd.String("command"),
								Args:    args,
								Reveal:  cmd.String("reveal"),
							}
							return gen.New().UpsertZedTasks(
								gen.WithExtraTasks(task),
								gen.WithZedTasksFile(cmd.String("file")),
							)(ctx)
						},
					},
				},
			},
		},
	}
}
