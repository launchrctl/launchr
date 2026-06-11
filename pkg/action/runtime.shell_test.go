package action

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shellResultActionYaml builds a shell action that declares a result schema and
// runs the given single-line bash script.
func shellResultActionYaml(script string) string {
	return "runtime:\n" +
		"  type: shell\n" +
		"  script: |\n" +
		"    " + script + "\n" +
		"action:\n" +
		"  title: T\n" +
		"  result:\n" +
		"    type: object\n"
}

func newShellResultAction(t *testing.T, script string) (*Action, *bytes.Buffer) {
	t.Helper()
	fsys := genFsTestMapActions(1, shellResultActionYaml(script), genPathTypeRoot)
	a, err := NewYAMLFromFS("test", fsys)
	require.NoError(t, err)
	require.NoError(t, a.EnsureLoaded())
	a.SetWorkDir(t.TempDir())

	out := &bytes.Buffer{}
	input := NewInput(a, nil, nil, launchr.NewBasicStreams(nil, out, io.Discard))
	input.SetValidated(true)
	require.NoError(t, a.SetInput(input))

	r := NewShellRuntime()
	r.(RuntimeLoggerAware).SetLogger(launchr.Log())
	a.SetRuntime(r)
	return a, out
}

func shellResult(a *Action) any {
	if rp, ok := a.Runtime().(RuntimeResultProvider); ok {
		return rp.Result()
	}
	return nil
}

// A result-schema shell action whose stdout is valid JSON: it is parsed into the
// structured result and not echoed raw to the terminal.
func TestShellRuntime_resultSchema_validJSON(t *testing.T) {
	a, out := newShellResultAction(t, `printf '%s' '{"ok":true}'`)
	require.NoError(t, a.Execute(context.Background()))
	assert.Equal(t, map[string]any{"ok": true}, shellResult(a))
	assert.Empty(t, out.String(), "valid JSON must not be echoed raw")
}

// Regression (#1): on JSON parse failure the shell runtime must display the raw
// output instead of swallowing it — mirroring the container runtime.
func TestShellRuntime_resultSchema_invalidJSON_displaysRaw(t *testing.T) {
	a, out := newShellResultAction(t, `printf '%s' 'plain text, not json'`)
	require.NoError(t, a.Execute(context.Background()))
	assert.Nil(t, shellResult(a))
	assert.Equal(t, "plain text, not json", out.String())
}

// Regression (#2): on action failure the captured stdout must be surfaced, not
// swallowed (display and parsing used to be gated on success).
func TestShellRuntime_resultSchema_failure_displaysRaw(t *testing.T) {
	a, out := newShellResultAction(t, `printf '%s' 'output before failure'; exit 3`)
	err := a.Execute(context.Background())
	require.Error(t, err)
	assert.Nil(t, shellResult(a))
	assert.Equal(t, "output before failure", out.String())
}
