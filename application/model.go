package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mishankov/pipefile/pipefile"
)

type stepStatus string

const (
	stepStatusReady    stepStatus = "ready"
	stepStatusRunning  stepStatus = "running"
	stepStatusDone     stepStatus = "done"
	stepStatusError    stepStatus = "error"
	stepStatusBlocked  stepStatus = "blocked"
	stepStatusCanceled stepStatus = "canceled"
)

type step struct {
	id, name       string
	dir            string
	outputBuff     *logBuffer
	status         stepStatus
	cmds, needs    []string
	remainingNeeds int
	dependents     []int
}

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func (b *logBuffer) WriteString(s string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.WriteString(s)
}

type model struct {
	steps          []step
	viewport       viewport.Model
	viewportReady  bool
	selectedStep   int
	ctx            context.Context
	cancel         context.CancelFunc
	pipelineFailed bool
	failureErr     error
	runningCount   int
	finishedCount  int
	runner         stepRunner
}

const headerHeight = 8

func newModel(parent context.Context, file pipefile.Pipefile, baseDir string) (*model, error) {
	steps, err := buildSteps(file, baseDir)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)

	return &model{
		steps:        steps,
		ctx:          ctx,
		cancel:       cancel,
		runner:       defaultStepRunner,
		selectedStep: 0,
	}, nil
}

func buildSteps(file pipefile.Pipefile, baseDir string) ([]step, error) {
	steps := make([]step, 0, len(file.Steps))
	indexByID := make(map[string]int, len(file.Steps))

	for i, fileStep := range file.Steps {
		if strings.TrimSpace(fileStep.Id) == "" {
			return nil, fmt.Errorf("step %d has empty id", i)
		}
		if _, exists := indexByID[fileStep.Id]; exists {
			return nil, fmt.Errorf("duplicate step id %q", fileStep.Id)
		}

		dir, err := resolveStepDir(fileStep.Dir, baseDir)
		if err != nil {
			return nil, fmt.Errorf("step %q dir %s", fileStep.Id, err)
		}

		vars := mergeVars(file.Vars, fileStep.Vars)
		cmds := expandCommands(fileStep.Cmds, vars)

		indexByID[fileStep.Id] = i
		steps = append(steps, step{
			id:             fileStep.Id,
			name:           fileStep.Name,
			dir:            dir,
			outputBuff:     &logBuffer{},
			status:         stepStatusReady,
			cmds:           cmds,
			needs:          append([]string(nil), fileStep.Needs...),
			remainingNeeds: len(fileStep.Needs),
		})
	}

	for i, fileStep := range file.Steps {
		for _, need := range fileStep.Needs {
			dependencyIndex, exists := indexByID[need]
			if !exists {
				return nil, fmt.Errorf("step %q needs unknown step %q", fileStep.Id, need)
			}
			steps[dependencyIndex].dependents = append(steps[dependencyIndex].dependents, i)
		}
	}

	if err := validateAcyclic(steps); err != nil {
		return nil, err
	}

	return steps, nil
}

func validateAcyclic(steps []step) error {
	remaining := make([]int, len(steps))
	for i := range steps {
		remaining[i] = steps[i].remainingNeeds
	}

	ready := make([]int, 0, len(steps))
	for i, count := range remaining {
		if count == 0 {
			ready = append(ready, i)
		}
	}

	visited := 0
	for len(ready) > 0 {
		index := ready[0]
		ready = ready[1:]
		visited++

		for _, dependent := range steps[index].dependents {
			remaining[dependent]--
			if remaining[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	if visited != len(steps) {
		return errors.New("pipefile has cyclic dependencies")
	}

	return nil
}

func resolveStepDir(rawDir, baseDir string) (string, error) {
	rawDir = strings.TrimSpace(rawDir)
	if rawDir == "" {
		return "", nil
	}

	dir := rawDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(baseDir, dir)
	}
	dir = filepath.Clean(dir)

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dir, fmt.Errorf("%q does not exist", dir)
		}
		return dir, err
	}
	if !info.IsDir() {
		return dir, fmt.Errorf("%q is not a directory", dir)
	}

	return dir, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.scheduleReadySteps(), doTick())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "left", "h":
			m.selectPreviousStep()
		case "right", "l", "tab":
			m.selectNextStep()
		}
		m.syncViewport()
	case stepFinishedMsg:
		cmds = append(cmds, m.handleStepFinished(msg))
	case TickMsg:
		m.syncViewport()
		if m.runningCount > 0 {
			cmds = append(cmds, doTick())
		}
	case tea.WindowSizeMsg:
		if !m.viewportReady {
			m.viewport = viewport.New(msg.Width, max(1, msg.Height-headerHeight))
			m.viewport.YPosition = headerHeight - 1
			m.viewportReady = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = max(1, msg.Height-headerHeight)
		}
		m.syncViewport()
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) scheduleReadySteps() tea.Cmd {
	if m.pipelineFailed {
		return nil
	}

	var cmds []tea.Cmd
	startedAny := false
	for i := range m.steps {
		if m.steps[i].status != stepStatusReady || m.steps[i].remainingNeeds != 0 {
			continue
		}

		m.steps[i].status = stepStatusRunning
		m.runningCount++
		startedAny = true
		cmds = append(cmds, m.runner(m.ctx, i, m.steps[i].dir, m.steps[i].cmds, m.steps[i].outputBuff, m.steps[i].outputBuff))
	}

	if startedAny {
		m.syncViewport()
	}

	return tea.Batch(cmds...)
}

func (m *model) handleStepFinished(msg stepFinishedMsg) tea.Cmd {
	if msg.index < 0 || msg.index >= len(m.steps) {
		return nil
	}

	current := &m.steps[msg.index]
	if current.status != stepStatusRunning {
		return nil
	}

	if m.runningCount > 0 {
		m.runningCount--
	}
	m.finishedCount++

	switch {
	case msg.err == nil:
		current.status = stepStatusDone
		for _, dependent := range current.dependents {
			if m.steps[dependent].remainingNeeds > 0 {
				m.steps[dependent].remainingNeeds--
			}
		}
		m.syncViewport()
		return m.scheduleReadySteps()
	case errors.Is(msg.err, context.Canceled) && m.pipelineFailed:
		current.status = stepStatusCanceled
	default:
		current.status = stepStatusError
		m.pipelineFailed = true
		m.failureErr = msg.err
		if m.cancel != nil {
			m.cancel()
		}
		for i := range m.steps {
			if m.steps[i].status == stepStatusReady {
				m.steps[i].status = stepStatusBlocked
			}
		}
	}

	m.syncViewport()
	return nil
}

func (m *model) syncViewport() {
	if len(m.steps) == 0 {
		return
	}

	if m.selectedStep < 0 {
		m.selectedStep = 0
	}
	if m.selectedStep >= len(m.steps) {
		m.selectedStep = len(m.steps) - 1
	}

	content := m.steps[m.selectedStep].outputBuff.String()
	if content == "" {
		content = fmt.Sprintf("No logs for %s.", m.steps[m.selectedStep].id)
	}
	if m.pipelineFailed && m.failureErr != nil {
		content = content + fmt.Sprintf("\n\nPipeline failed: %v", m.failureErr)
	}

	m.viewport.SetContent(content)
	if m.steps[m.selectedStep].status == stepStatusRunning {
		m.viewport.GotoBottom()
	}
}

func (m *model) selectPreviousStep() {
	if len(m.steps) == 0 {
		return
	}
	m.selectedStep--
	if m.selectedStep < 0 {
		m.selectedStep = len(m.steps) - 1
	}
}

func (m *model) selectNextStep() {
	if len(m.steps) == 0 {
		return
	}
	m.selectedStep = (m.selectedStep + 1) % len(m.steps)
}

func (m *model) View() string {
	if len(m.steps) == 0 {
		return "No steps configured.\nPress q to quit.\n"
	}

	cards := make([]string, 0, len(m.steps))
	for i, step := range m.steps {
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(1).
			Width(22).
			Height(3).
			MarginRight(1).
			Align(lipgloss.Center, lipgloss.Center).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3f3f46"))

		switch step.status {
		case stepStatusReady:
			style = style.Background(lipgloss.Color("#555"))
		case stepStatusRunning:
			style = style.Background(lipgloss.Color("#3b82f6"))
		case stepStatusDone:
			style = style.Background(lipgloss.Color("#16a34a"))
		case stepStatusError:
			style = style.Background(lipgloss.Color("#dc2626"))
		case stepStatusBlocked:
			style = style.Background(lipgloss.Color("#a16207"))
		case stepStatusCanceled:
			style = style.Background(lipgloss.Color("#7c3aed"))
		}

		if i == m.selectedStep {
			style = style.BorderForeground(lipgloss.Color("#facc15"))
		}

		title := step.name
		if title == "" {
			title = step.id
		}
		cards = append(cards, style.Render(fmt.Sprintf("%s\n%s", title, step.status)))
	}

	statusLine := fmt.Sprintf(
		"Selected: %s | left/h previous | right/l/tab next | q quit",
		m.steps[m.selectedStep].id,
	)
	if m.pipelineFailed && m.failureErr != nil {
		statusLine = fmt.Sprintf("%s | failed: %v", statusLine, m.failureErr)
	}

	return "\n" + lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, cards...),
		"",
		statusLine,
		m.viewport.View(),
	)
}
