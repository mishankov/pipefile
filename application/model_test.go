package application

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishankov/pipefile/pipefile"
)

func TestBuildStepsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file pipefile.Pipefile
		err  string
	}{
		{
			name: "missing dependency",
			file: pipefile.Pipefile{Steps: []pipefile.PipeStep{
				{Id: "test", Needs: []string{"build"}},
			}},
			err: `step "test" needs unknown step "build"`,
		},
		{
			name: "duplicate id",
			file: pipefile.Pipefile{Steps: []pipefile.PipeStep{
				{Id: "build"},
				{Id: "build"},
			}},
			err: `duplicate step id "build"`,
		},
		{
			name: "cycle",
			file: pipefile.Pipefile{Steps: []pipefile.PipeStep{
				{Id: "build", Needs: []string{"deploy"}},
				{Id: "deploy", Needs: []string{"build"}},
			}},
			err: "pipefile has cyclic dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSteps(tt.file, "")
			if err == nil || err.Error() != tt.err {
				t.Fatalf("expected error %q, got %v", tt.err, err)
			}
		})
	}
}

func TestBuildStepsRejectsMissingDir(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	file := pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build", Dir: missingDir}},
	}

	_, err := buildSteps(file, "")
	want := `step "build" dir "` + filepath.Clean(missingDir) + `" does not exist`
	if err == nil || err.Error() != want {
		t.Fatalf("expected error %q, got %v", want, err)
	}
}

func TestBuildStepsRejectsFileDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file := pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build", Dir: filePath}},
	}

	_, err := buildSteps(file, "")
	want := `step "build" dir "` + filepath.Clean(filePath) + `" is not a directory`
	if err == nil || err.Error() != want {
		t.Fatalf("expected error %q, got %v", want, err)
	}
}

func TestBuildStepsResolvesRelativeDirAgainstBaseDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	wantDir := filepath.Join(baseDir, "frontend")
	if err := os.Mkdir(wantDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	steps, err := buildSteps(pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build", Dir: "./frontend"}},
	}, baseDir)
	if err != nil {
		t.Fatalf("buildSteps() error = %v", err)
	}

	if got := steps[0].dir; got != filepath.Clean(wantDir) {
		t.Fatalf("unexpected dir: got %q want %q", got, filepath.Clean(wantDir))
	}
}

func TestBuildStepsWithoutDirLeavesExecutionDirEmpty(t *testing.T) {
	t.Parallel()

	steps, err := buildSteps(pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build"}},
	}, "")
	if err != nil {
		t.Fatalf("buildSteps() error = %v", err)
	}

	if steps[0].dir != "" {
		t.Fatalf("expected empty dir, got %q", steps[0].dir)
	}
}

func TestScheduleSingleStep(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build"}},
	})

	var starts []int
	m.runner = recordRunner(&starts)

	cmd := m.Init()
	if cmd != nil {
		_ = cmd()
	}

	if got := []stepStatus{m.steps[0].status}; !reflect.DeepEqual(got, []stepStatus{stepStatusRunning}) {
		t.Fatalf("unexpected statuses: %v", got)
	}
	if !reflect.DeepEqual(starts, []int{0}) {
		t.Fatalf("unexpected start order: %v", starts)
	}
}

func TestScheduleReadyStepsPassesResolvedDirToRunner(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	wantDir := filepath.Join(baseDir, "frontend")
	if err := os.Mkdir(wantDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	m := mustTestModelWithBaseDir(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{{Id: "build", Dir: "./frontend"}},
	}, baseDir)

	var starts []int
	var dirs []string
	m.runner = func(_ context.Context, index int, dir string, _ []string, _, _ io.Writer) tea.Cmd {
		starts = append(starts, index)
		dirs = append(dirs, dir)
		return nil
	}

	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}

	expectStarts(t, starts, []int{0})
	if !reflect.DeepEqual(dirs, []string{filepath.Clean(wantDir)}) {
		t.Fatalf("unexpected dirs: got %v want %v", dirs, []string{filepath.Clean(wantDir)})
	}
}

func TestLinearDependenciesRespectNeeds(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{
			{Id: "build"},
			{Id: "test", Needs: []string{"build"}},
			{Id: "deploy", Needs: []string{"test"}},
		},
	})

	var starts []int
	m.runner = recordRunner(&starts)
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}
	expectStarts(t, starts, []int{0})

	m = applyUpdate(t, m, stepFinishedMsg{index: 0})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusRunning, stepStatusReady})
	expectStarts(t, starts, []int{0, 1})

	m = applyUpdate(t, m, stepFinishedMsg{index: 1})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusDone, stepStatusRunning})
	expectStarts(t, starts, []int{0, 1, 2})

	m = applyUpdate(t, m, stepFinishedMsg{index: 2})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusDone, stepStatusDone})
}

func TestParallelFanOutStartsInConfigOrder(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{
			{Id: "build"},
			{Id: "lint", Needs: []string{"build"}},
			{Id: "test", Needs: []string{"build"}},
		},
	})

	var starts []int
	m.runner = recordRunner(&starts)
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}

	m = applyUpdate(t, m, stepFinishedMsg{index: 0})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusRunning, stepStatusRunning})
	expectStarts(t, starts, []int{0, 1, 2})
}

func TestParallelFanInWaitsForAllNeeds(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{
			{Id: "build"},
			{Id: "lint", Needs: []string{"build"}},
			{Id: "test", Needs: []string{"build"}},
			{Id: "deploy", Needs: []string{"lint", "test"}},
		},
	})

	var starts []int
	m.runner = recordRunner(&starts)
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}

	m = applyUpdate(t, m, stepFinishedMsg{index: 0})
	m = applyUpdate(t, m, stepFinishedMsg{index: 1})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusDone, stepStatusRunning, stepStatusReady})
	expectStarts(t, starts, []int{0, 1, 2})

	m = applyUpdate(t, m, stepFinishedMsg{index: 2})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusDone, stepStatusDone, stepStatusRunning})
	expectStarts(t, starts, []int{0, 1, 2, 3})
}

func TestFailureStopsPipelineAndBlocksDependents(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{
			{Id: "build"},
			{Id: "lint", Needs: []string{"build"}},
			{Id: "test", Needs: []string{"build"}},
			{Id: "deploy", Needs: []string{"test"}},
		},
	})

	var starts []int
	m.runner = recordRunner(&starts)
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}
	m = applyUpdate(t, m, stepFinishedMsg{index: 0})

	m = applyUpdate(t, m, stepFinishedMsg{index: 1, err: errors.New("lint failed")})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusError, stepStatusRunning, stepStatusBlocked})

	m = applyUpdate(t, m, stepFinishedMsg{index: 2, err: context.Canceled})
	expectStatuses(t, m.steps, []stepStatus{stepStatusDone, stepStatusError, stepStatusCanceled, stepStatusBlocked})
}

func TestSelectedStepNavigationUpdatesViewport(t *testing.T) {
	t.Parallel()

	m := mustTestModel(t, pipefile.Pipefile{
		Steps: []pipefile.PipeStep{
			{Id: "build"},
			{Id: "test"},
		},
	})
	m.viewport = viewport.New(80, 20)
	m.viewportReady = true
	m.steps[0].outputBuff.WriteString("build logs")
	m.steps[1].outputBuff.WriteString("test logs")
	m.syncViewport()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := updated.(*model)
	if next.selectedStep != 1 {
		t.Fatalf("expected selected step 1, got %d", next.selectedStep)
	}
	if got := next.viewport.View(); !contains(got, "test logs") {
		t.Fatalf("expected viewport to show selected logs, got %q", got)
	}
}

func mustTestModel(t *testing.T, file pipefile.Pipefile) *model {
	t.Helper()

	return mustTestModelWithBaseDir(t, file, "")
}

func mustTestModelWithBaseDir(t *testing.T, file pipefile.Pipefile, baseDir string) *model {
	t.Helper()

	m, err := newModel(context.Background(), file, baseDir)
	if err != nil {
		t.Fatalf("newModel() error = %v", err)
	}

	return m
}

func recordRunner(starts *[]int) stepRunner {
	return func(_ context.Context, index int, _ string, _ []string, _, _ io.Writer) tea.Cmd {
		*starts = append(*starts, index)
		return nil
	}
}

func applyUpdate(t *testing.T, m *model, msg tea.Msg) *model {
	t.Helper()

	nextModel, cmd := m.Update(msg)
	next := nextModel.(*model)
	if cmd != nil {
		_ = cmd()
	}
	return next
}

func expectStatuses(t *testing.T, steps []step, want []stepStatus) {
	t.Helper()

	got := make([]stepStatus, 0, len(steps))
	for _, step := range steps {
		got = append(got, step.status)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected statuses: got %v want %v", got, want)
	}
}

func expectStarts(t *testing.T, got, want []int) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected start order: got %v want %v", got, want)
	}
}

func contains(s, needle string) bool {
	return strings.Contains(s, needle)
}
