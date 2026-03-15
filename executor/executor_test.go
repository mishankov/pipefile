package executor

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestExecuteManyStopsOnFirstFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := new(Executor).ExecuteMany(context.Background(), &stdout, &stdout, successCommand("first"), failureCommand(), successCommand("second"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := stdout.String(); got != "first\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestExecuteManyRespectsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var stdout bytes.Buffer
	err := new(Executor).ExecuteMany(ctx, &stdout, &stdout, longRunningCommand())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func successCommand(message string) string {
	if runtime.GOOS == "windows" {
		return "echo " + message
	}
	return "printf '" + message + "\\n'"
}

func failureCommand() string {
	if runtime.GOOS == "windows" {
		return "exit /b 1"
	}
	return "exit 1"
}

func longRunningCommand() string {
	if runtime.GOOS == "windows" {
		return "powershell -Command Start-Sleep -Seconds 5"
	}
	return "sleep 5"
}
