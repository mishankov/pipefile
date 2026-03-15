package application

import (
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishankov/pipefile/executor"
)

type TickMsg time.Time

type stepFinishedMsg struct {
	index int
	err   error
}

type stepRunner func(ctx context.Context, index int, dir string, cmds []string, stdout, stderr io.Writer) tea.Cmd

func doTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func defaultStepRunner(ctx context.Context, index int, dir string, cmds []string, stdout, stderr io.Writer) tea.Cmd {
	return func() tea.Msg {
		err := new(executor.Executor).ExecuteMany(ctx, stdout, stderr, dir, cmds...)
		return stepFinishedMsg{index: index, err: err}
	}
}
