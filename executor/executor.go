package executor

import (
	"context"
	"io"
	"log/slog"
	"os/exec"
)

type Executor struct {
	Finished bool
}

func (e *Executor) Execute(ctx context.Context, stdout, stderr io.Writer, command string) {
	cmd := exec.Command("/bin/sh", "-c", command)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("error", "error", err)
	}

	e.Finished = true
}

func (e *Executor) ExecuteMany(stdout, stderr io.Writer, commands ...string) {
	for _, command := range commands {
		cmd := exec.Command("/bin/sh", "-c", command)

		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()
		if err != nil {
			slog.Error("error", "error", err)
		}
	}

	e.Finished = true
}
