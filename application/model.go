package application

import (
	"bytes"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mishankov/pipefile/executor"
	"github.com/mishankov/pipefile/pipefile"
)

type stepStatus string

const (
	stepStatusReady   stepStatus = "ready"
	stepStatusRunning stepStatus = "running"
	stepStatusDone    stepStatus = "done"
	stepStatusError   stepStatus = "error"
)

type step struct {
	id, name    string
	outputBuff  *bytes.Buffer
	status      stepStatus
	cmds, needs []string
	executor    *executor.Executor
}

type model struct {
	steps         []step
	viewport      viewport.Model
	viewportReady bool
}

type TickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(10*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func newModel(file pipefile.Pipefile) model {
	m := model{}
	for _, fileStep := range file.Steps {
		s := step{
			id:         fileStep.Id,
			name:       fileStep.Name,
			outputBuff: &bytes.Buffer{},
			status:     stepStatusReady,
			cmds:       fileStep.Cmds,
			needs:      fileStep.Needs,
			executor:   &executor.Executor{},
		}
		m.steps = append(m.steps, s)
	}

	return m
}

func (m model) Init() tea.Cmd {
	return determineNextStep(m)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case hasNextStepMsg:
		m.steps[msg.number].status = stepStatusRunning
		cmds = append(cmds, runStep(msg.number, m))

	case stepRunningMessage:
		cmds = append(cmds, doTick())

	case finishMsg:
		content := ""
		for _, step := range m.steps {
			content += step.outputBuff.String() + "\n"
		}
		m.viewport.SetContent(content)

	case TickMsg:
		sendTick := false
		for i, step := range m.steps {
			if step.status == stepStatusRunning {
				if step.executor.Finished {
					m.steps[i].status = stepStatusDone
					return m, determineNextStep(m)
				} else {
					sendTick = true
					m.viewport.SetContent(step.outputBuff.String())
					m.viewport.GotoBottom()
				}
			}
		}

		if sendTick {
			cmds = append(cmds, doTick())
		}
	case tea.WindowSizeMsg:
		if !m.viewportReady {
			m.viewport = viewport.New(msg.Width, 20)
			m.viewport.YPosition = 20
			m.viewportReady = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = 20
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	s := ""

	// Steps cards
	stepsStyles := []string{}
	for _, step := range m.steps {
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(1).
			Width(22).
			MarginRight(5).
			Align(lipgloss.Center)

		switch step.status {
		case stepStatusReady:
			style = style.Background(lipgloss.Color("#555"))
		case stepStatusRunning:
			style = style.Background(lipgloss.Color("#55f"))
		case stepStatusDone:
			style = style.Background(lipgloss.Color("#5f5"))
		case stepStatusError:
			style = style.Background(lipgloss.Color("#f55"))
		}

		stepsStyles = append(stepsStyles, style.Render(step.name+" "+string(step.status)))
	}
	s += lipgloss.JoinHorizontal(lipgloss.Top, stepsStyles...)

	s += "\n"

	// Current running step buffer
	for _, step := range m.steps {
		if step.status == stepStatusRunning {
			// str := step.outputBuff.String()
			// if len(strings.Split(str, "\n")) > 20 {
			// 	str = strings.Join(strings.Split(str, "\n")[len(strings.Split(str, "\n"))-20:], "\n")
			// }

			// logs := lipgloss.NewStyle().
			// 	Background(lipgloss.Color("#222")).
			// 	Render(str)
			// s += lipgloss.JoinVertical(lipgloss.Left, logs)

			s += m.viewport.View()
		}

		if step.status == stepStatusDone {
			if m.viewportReady {
				s += m.viewport.View()
			}
		}
	}

	s += "\nPress q to quit.\n"

	return s
}
