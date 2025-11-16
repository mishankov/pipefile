package application

type finishMsg struct{}

type doneStepMsg struct {
	number int
	id     string
}

type hasNextStepMsg struct {
	number int
	id     string
}

type stepRunningMessage struct {
	number int
	id     string
}
