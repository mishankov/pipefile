package pipefile

import (
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestUnmarshalVars(t *testing.T) {
	t.Parallel()

	input := `
[vars]
global = "some value"

[[steps]]
id = "build"
vars = { stepVar = "wow" }
cmds = ["echo @{stepVar} @{global}"]
`

	var file Pipefile
	if err := toml.Unmarshal([]byte(input), &file); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got := file.Vars; !reflect.DeepEqual(got, map[string]string{"global": "some value"}) {
		t.Fatalf("unexpected top-level vars: got %v", got)
	}

	if len(file.Steps) != 1 {
		t.Fatalf("unexpected step count: got %d want 1", len(file.Steps))
	}

	if got := file.Steps[0].Vars; !reflect.DeepEqual(got, map[string]string{"stepVar": "wow"}) {
		t.Fatalf("unexpected step vars: got %v", got)
	}
}
