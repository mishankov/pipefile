package application

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishankov/pipefile/pipefile"
)

type Application struct {
	pipeFile pipefile.Pipefile
	baseDir  string
}

func New(pipeFile pipefile.Pipefile, baseDir string) *Application {
	return &Application{pipeFile: pipeFile, baseDir: baseDir}
}

func (a *Application) Run(ctx context.Context) error {
	model, err := newModel(ctx, a.pipeFile, a.baseDir)
	if err != nil {
		return err
	}

	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}

	if _, err := tea.NewProgram(model, opts...).Run(); err != nil {
		fmt.Printf("Uh oh, there was an error: %v\n", err)
		return err
	}

	return nil
}
