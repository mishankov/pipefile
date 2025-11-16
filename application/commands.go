package application

import (
	tea "github.com/charmbracelet/bubbletea"
)

func runStep(number int, m model) func() tea.Msg {
	return func() tea.Msg {
		go m.steps[number].executor.ExecuteMany(
			m.steps[number].outputBuff,
			m.steps[number].outputBuff,
			m.steps[number].cmds...,
		)

		return stepRunningMessage{number: number, id: m.steps[number].id}
	}
}

func determineNextStep(m model) func() tea.Msg {
	return func() tea.Msg {
		for i, step := range m.steps {
			if step.status == stepStatusReady {
				return hasNextStepMsg{i, step.id}
			}
		}

		return finishMsg{}
	}
}
