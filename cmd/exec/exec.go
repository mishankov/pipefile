package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
)

type myWriter struct{}

func (w *myWriter) Write(p []byte) (n int, err error) {
	slog.Info(string(p))
	return len(p), nil
}

func main() {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "docker build .")

	// w := &bytes.Buffer{}

	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("error", "error", err)
	}
}
