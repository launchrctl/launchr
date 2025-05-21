package action

import (
	"math/rand"
	"path"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/launchrctl/launchr/internal/launchr"
)

type genPathType int

const (
	genPathTypeValid     genPathType = iota // genPathTypeValid is a valid actions path
	genPathTypeArbitrary                    // genPathTypeArbitrary is a random string without actions directory.
	genPathTypeGHActions                    // genPathTypeGHActions is an incorrect hidden path but with actions directory.
)

func genActionPath(d int, pathType genPathType) string {
	elems := make([]string, 0, d+3)
	for i := 0; i < d; i++ {
		elems = append(elems, launchr.GetRandomString(4))
	}
	switch pathType {
	case genPathTypeValid:
		elems = append(elems, actionsDirname, launchr.GetRandomString(4))
	case genPathTypeGHActions:
		elems = append(elems, ".github", actionsDirname, launchr.GetRandomString(4))
	case genPathTypeArbitrary:
		// Do nothing.
	default:
		// Do nothing.
	}
	elems = append(elems, "action.yaml")
	return path.Join(elems...)
}

func genFsTestMapActions(num int, str string, pathType genPathType) fstest.MapFS {
	m := make(fstest.MapFS)
	depth := rand.Intn(5) + 1 //nolint:gosec // G404: We don't need strong random for tests.

	for i := 0; i < num; i++ {
		fa := genActionPath(depth, pathType)
		m[fa] = &fstest.MapFile{Data: []byte(str)}
	}
	return m
}

func mergeFsTestMaps(maps ...fstest.MapFS) fstest.MapFS {
	m := make(fstest.MapFS)
	for _, mm := range maps {
		for k, v := range mm {
			m[k] = v
		}
	}
	return m
}

type errTestAny struct{}

func (err errTestAny) Error() string {
	return "test error that matches any error"
}

func (err errTestAny) Is(cmp error) bool {
	return cmp != nil
}

func assertIsSameError(t *testing.T, exp error, act error) {
	if exp == (errTestAny{}) {
		assert.Error(t, act)
	} else if assert.ObjectsAreEqual(reflect.TypeOf(exp), reflect.TypeOf(act)) {
		assert.Equal(t, exp, act)
	} else {
		assert.ErrorIs(t, act, exp)
	}
}

// TestCaseValueProcessor is a common test case behavior for [ValueProcessor].
type TestCaseValueProcessor struct {
	Name    string
	Yaml    string
	ErrInit error
	ErrProc error
	Args    InputParams
	Opts    InputParams
	ExpArgs InputParams
	ExpOpts InputParams
}

// Test runs the test for [ValueProcessor].
func (tt TestCaseValueProcessor) Test(t *testing.T, am Manager) {
	a := NewFromYAML(tt.Name, []byte(tt.Yaml))
	// Init processors in the action.
	err := a.SetProcessors(am.GetValueProcessors())
	assertIsSameError(t, tt.ErrInit, err)
	if tt.ErrInit != nil {
		return
	}
	// Run processors.
	input := NewInput(a, tt.Args, tt.Opts, nil)
	err = a.SetInput(input)
	assertIsSameError(t, tt.ErrProc, err)
	if tt.ErrProc != nil {
		return
	}
	// Test input is processed.
	input = a.Input()
	if tt.ExpArgs == nil {
		tt.ExpArgs = InputParams{}
	}
	if tt.ExpOpts == nil {
		tt.ExpOpts = InputParams{}
	}
	assert.Equal(t, tt.ExpArgs, input.Args())
	assert.Equal(t, tt.ExpOpts, input.Opts())
}
