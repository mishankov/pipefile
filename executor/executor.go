package executor

import (
	"context"
	"io"
	"os/exec"
	"runtime"
)

type Executor struct{}

func (e *Executor) Execute(ctx context.Context, stdout, stderr io.Writer, dir, command string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}

	if dir != "" {
		cmd.Dir = dir
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}

func (e *Executor) ExecuteMany(ctx context.Context, stdout, stderr io.Writer, dir string, commands ...string) error {
	for _, command := range commands {
		if err := e.Execute(ctx, stdout, stderr, dir, command); err != nil {
			return err
		}
	}

	return nil
}
