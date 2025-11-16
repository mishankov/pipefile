package application

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishankov/pipefile/pipefile"
)

type Application struct {
	pipeFile pipefile.Pipefile
}

func New(pipeFile pipefile.Pipefile) *Application {
	return &Application{pipeFile: pipeFile}
}

func (a *Application) Run(ctx context.Context) error {
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),       // to use full screen
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	}

	if _, err := tea.NewProgram(newModel(a.pipeFile), opts...).Run(); err != nil {
		fmt.Printf("Uh oh, there was an error: %v\n", err)
		return err
	}

	return nil
}
