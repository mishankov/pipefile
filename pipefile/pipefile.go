package pipefile

type PipeStep struct {
	Id    string
	Name  string
	Needs []string
	Dir   string
	Cmds  []string
}

type Pipefile struct {
	Steps []PipeStep
}
