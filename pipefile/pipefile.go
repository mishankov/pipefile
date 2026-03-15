package pipefile

type PipeStep struct {
	Id    string            `toml:"id"`
	Name  string            `toml:"name"`
	Needs []string          `toml:"needs"`
	Dir   string            `toml:"dir"`
	Cmds  []string          `toml:"cmds"`
	Vars  map[string]string `toml:"vars"`
}

type Pipefile struct {
	Steps []PipeStep        `toml:"steps"`
	Vars  map[string]string `toml:"vars"`
}
