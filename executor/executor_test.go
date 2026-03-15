package executor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestExecuteManyStopsOnFirstFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := new(Executor).ExecuteMany(context.Background(), &stdout, &stdout, "", successCommand("first"), failureCommand(), successCommand("second"))
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
	err := new(Executor).ExecuteMany(ctx, &stdout, &stdout, "", longRunningCommand())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestExecuteRunsCommandInProvidedDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := new(Executor).Execute(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, writeFileCommand("marker.txt", "hello"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "marker.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != "hello\n" {
		t.Fatalf("unexpected file contents: %q", got)
	}
}

func TestExecuteManyUsesSameDirectoryForAllCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := new(Executor).ExecuteMany(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		dir,
		writeFileCommand("first.txt", "first"),
		writeFileCommand("second.txt", "second"),
	)
	if err != nil {
		t.Fatalf("ExecuteMany() error = %v", err)
	}

	for name, want := range map[string]string{
		"first.txt":  "first\n",
		"second.txt": "second\n",
	} {
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", name, readErr)
		}
		if got := string(data); got != want {
			t.Fatalf("unexpected contents for %q: got %q want %q", name, got, want)
		}
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

func writeFileCommand(filename, content string) string {
	if runtime.GOOS == "windows" {
		return "echo " + content + " > " + filename
	}
	return "printf '" + content + "\\n' > " + filename
}
