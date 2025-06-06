package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/driver"
	containermock "github.com/launchrctl/launchr/pkg/driver/mocks"
)

const containerNamePrefix = "test_prefix_"

const (
	mockFnImageEnsure     = "ImageEnsure"
	mockFnContainerCreate = "ContainerCreate"
	mockFnContainerStart  = "ContainerStart"
	mockFnContainerRemove = "ContainerRemove"
	mockFnImageRemove     = "ImageRemove"
)

type eqImageOpts struct {
	x driver.ImageOptions
}

func (e eqImageOpts) Matches(x any) bool {
	return assert.ObjectsAreEqual(e.x, x.(driver.ImageOptions))
}

func (e eqImageOpts) String() string {
	return fmt.Sprintf("is equal to %v (%T)", e.x, e.x)
}

var cfgImgRes = LaunchrConfigImageBuildResolver{dummyCfg()}

func dummyCfg() launchr.Config {
	cfgRoot := fstest.MapFS{"config.yaml": &fstest.MapFile{Data: []byte(cfgYaml)}}
	return launchr.ConfigFromFS(cfgRoot)
}

func prepareContainerTestSuite(t *testing.T) (*gomock.Controller, *containermock.MockContainerRuntime, *runtimeContainer) {
	ctrl := gomock.NewController(t)
	d := containermock.NewMockContainerRuntime(ctrl)
	d.EXPECT().Close()
	r := &runtimeContainer{crt: d, rtype: "mock"}
	r.SetLogger(launchr.Log())
	r.SetTerm(launchr.Term())
	r.AddImageBuildResolver(cfgImgRes)
	r.SetContainerNameProvider(ContainerNameProvider{Prefix: containerNamePrefix})

	return ctrl, d, r
}

func testContainerAction(cdef *DefRuntimeContainer) *Action {
	if cdef == nil {
		cdef = &DefRuntimeContainer{
			Image:   "myimage",
			Command: []string{"my", "cmd"},
			ExtraHosts: []string{
				"my:host1",
				"my:host2",
			},
			Env: []string{
				"env1=var1",
				"env2=var2",
			},
		}
	}
	a := New(
		StringID("test"),
		&Definition{
			Action: &DefAction{},
			Runtime: &DefRuntime{
				Type:      runtimeTypeContainer,
				Container: cdef,
			},
		},
		NewDiscoveryFS(nil, launchr.MustAbs("test")),
		"my/action/test/action.yaml",
	)
	return a
}

func Test_ContainerExec_imageEnsure(t *testing.T) {
	t.Parallel()

	actLoc := testContainerAction(&DefRuntimeContainer{
		Image: "build:local",
		Build: &driver.BuildDefinition{
			Context: ".",
		},
	})
	type testCase struct {
		name     string
		action   *DefRuntimeContainer
		expBuild *driver.BuildDefinition
		ret      []any
	}

	imgFn := func(s driver.ImageStatus, pstr string, err error) []any {
		var p io.ReadCloser
		if pstr != "" {
			p = io.NopCloser(strings.NewReader(pstr))
		}
		var r *driver.ImageStatusResponse
		if s != -1 {
			r = &driver.ImageStatusResponse{
				Status:   s,
				Progress: &driver.ImageProgressStream{ReadCloser: p},
			}
		}
		return []any{r, err}
	}

	aconf := actLoc.RuntimeDef().Container
	tts := []testCase{
		{
			"image exists",
			&DefRuntimeContainer{Image: "exists"},
			nil,
			imgFn(driver.ImageExists, "", nil),
		},
		{
			"image pulled",
			&DefRuntimeContainer{Image: "pull"},
			nil,
			imgFn(driver.ImagePull, `"Successfully pulled image"`, nil),
		},
		{
			"image pulled error",
			&DefRuntimeContainer{Image: "pull"},
			nil,
			imgFn(
				driver.ImagePull,
				"fake pull error",
				fmt.Errorf("fake pull error"),
			),
		},
		{
			"image build local",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(driver.ImageBuild, `Successfully built image "local"`, nil),
		},
		{
			"image build local error",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(
				driver.ImageBuild,
				"fake build error",
				fmt.Errorf("fake build error"),
			),
		},
		{
			"image build config",
			&DefRuntimeContainer{Image: "build:config"},
			cfgImgRes.ImageBuildInfo("build:config"),
			imgFn(driver.ImageBuild, `Successfully built image "config"`, nil),
		},
		{
			"container runtime error",
			&DefRuntimeContainer{Image: ""},
			nil,
			imgFn(-1, "", fmt.Errorf("incorrect image")),
		},
	}

	for _, tt := range tts {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl, d, r := prepareContainerTestSuite(t)
			defer ctrl.Finish()
			defer r.Close()
			ctx := context.Background()
			act := testContainerAction(tt.action)
			act.input = NewInput(act, nil, nil, launchr.NoopStreams())
			run := act.RuntimeDef().Container
			imgOpts := driver.ImageOptions{Name: run.Image, Build: tt.expBuild}
			d.EXPECT().
				ImageEnsure(ctx, eqImageOpts{imgOpts}).
				Return(tt.ret...)
			err := r.imageEnsure(ctx, act)
			assert.Equal(t, tt.ret[1], err)
		})
	}
}

func Test_ContainerExec_imageRemove(t *testing.T) {
	t.Parallel()

	actLoc := testContainerAction(&DefRuntimeContainer{
		Image: "build:local",
		Build: &driver.BuildDefinition{
			Context: ".",
		},
	})
	type testCase struct {
		name     string
		action   *DefRuntimeContainer
		expBuild *driver.BuildDefinition
		ret      []any
	}

	tts := []testCase{
		{
			"image removed",
			actLoc.RuntimeDef().Container,
			nil,
			[]any{&driver.ImageRemoveResponse{Status: driver.ImageRemoved}, nil},
		},
		{
			"failed to remove",
			&DefRuntimeContainer{Image: "failed"},
			nil,
			[]any{nil, fmt.Errorf("failed to remove")},
		},
	}

	for _, tt := range tts {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl, d, r := prepareContainerTestSuite(t)
			ctx := context.Background()

			defer ctrl.Finish()
			defer r.crt.Close()

			act := testContainerAction(tt.action)
			act.input = NewInput(act, nil, nil, launchr.NoopStreams())

			run := act.RuntimeDef().Container
			imgOpts := driver.ImageRemoveOptions{Force: true}
			d.EXPECT().
				ImageRemove(ctx, run.Image, gomock.Eq(imgOpts)).
				Return(tt.ret...)
			err := r.imageRemove(ctx, act)

			assert.Equal(t, err, tt.ret[1])
		})
	}
}

func Test_ContainerExec_createContainerDef(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		newAct func(*runtimeContainer) *Action
		exp    driver.ContainerDefinition
	}

	baseRes := driver.ContainerDefinition{
		Image:      "myimage",
		WorkingDir: containerHostMount,
		ExtraHosts: []string{
			"my:host1",
			"my:host2",
		},
		Env: []string{
			"env1=var1",
			"env2=var2",
		},
		User: getCurrentUser(),
	}
	defaultCmd := []string{"my", "cmd"}
	var defaultEntrypoint []string
	actionDir := launchr.MustAbs("my/action/test")

	tts := []testCase{
		{
			"local binds, default working dir, stdin, tty",
			func(_ *runtimeContainer) *Action {
				a := testContainerAction(nil)
				streams := launchr.NewBasicStreams(io.NopCloser(strings.NewReader("")), io.Discard, io.Discard)
				streams.In().SetIsTerminal(true)
				input := NewInput(a, nil, nil, streams)
				input.SetValidated(true)
				_ = a.SetInput(input)
				return a
			},
			driver.ContainerDefinition{
				ContainerName: launchr.GetRandomString(4),
				Command:       defaultCmd,
				Entrypoint:    defaultEntrypoint,
				Binds: []string{
					launchr.MustAbs("./") + ":" + containerHostMount,
					actionDir + ":" + containerActionMount,
				},
				Streams: driver.ContainerStreamsOptions{
					Stdin:  true,
					Stdout: true,
					Stderr: true,
					TTY:    true,
				},
			},
		},
		{
			"local binds, different working dir, no stdin, no tty",
			func(_ *runtimeContainer) *Action {
				a := testContainerAction(nil)
				input := NewInput(a, nil, nil, launchr.NewBasicStreams(nil, io.Discard, io.Discard))
				input.SetValidated(true)
				_ = a.SetInput(input)
				a.def.WD = "../myactiondir"
				return a
			},
			driver.ContainerDefinition{
				ContainerName: launchr.GetRandomString(4),
				Command:       defaultCmd,
				Entrypoint:    defaultEntrypoint,
				Binds: []string{
					launchr.MustAbs("../myactiondir") + ":" + containerHostMount,
					actionDir + ":" + containerActionMount,
				},
				Streams: driver.ContainerStreamsOptions{
					Stdout: true,
					Stderr: true,
				},
			},
		},
		{
			"remote volumes, no attach, remote runtime, override entrypoint, override cmd",
			func(r *runtimeContainer) *Action {
				a := testContainerAction(nil)
				args, _ := ArgsPosToNamed(a, []string{"arg1", "arg2"})
				input := NewInput(a, args, nil, launchr.NoopStreams())
				input.SetValidated(true)
				_ = a.SetInput(input)
				r.isRemoteRuntime = true
				r.entrypointSet = true
				r.entrypoint = "/my/entrypoint"
				r.exec = true
				return a
			},
			driver.ContainerDefinition{
				ContainerName: launchr.GetRandomString(4),
				Command:       []string{"arg1", "arg2"},
				Entrypoint:    []string{"/my/entrypoint"},
				Volumes: containerAnonymousVolumes(
					containerHostMount,
					containerActionMount,
				),
			},
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl, _, r := prepareContainerTestSuite(t)
			defer ctrl.Finish()
			defer r.Close()

			a := tt.newAct(r)
			a.SetRuntime(r)
			cname := tt.exp.ContainerName
			tt.exp.Image = baseRes.Image
			tt.exp.WorkingDir = baseRes.WorkingDir
			tt.exp.ExtraHosts = baseRes.ExtraHosts
			tt.exp.Env = baseRes.Env
			tt.exp.User = baseRes.User
			runCfg := r.createContainerDef(a, cname)
			assert.Equal(t, tt.exp, runCfg)
		})
	}
}

type mockCallInfo struct {
	fn       string
	minTimes int
	maxTimes int
	args     []any
	ret      []any
	prepFn   func(*mockCallInfo)
}

func Test_ContainerExec(t *testing.T) {
	t.Parallel()

	cid := "cid"
	act := testContainerAction(nil)
	runConf := act.RuntimeDef().Container
	imgBuild := &driver.ImageStatusResponse{Status: driver.ImageExists}
	nprv := ContainerNameProvider{Prefix: containerNamePrefix}

	type testCase struct {
		name   string
		steps  []mockCallInfo
		expErr error
	}

	opts := driver.ContainerDefinition{
		ContainerName: nprv.Get(act.ID),
		Command:       runConf.Command,
		Image:         runConf.Image,
		ExtraHosts:    runConf.ExtraHosts,
		Binds: []string{
			launchr.MustAbs(act.WorkDir()) + ":" + containerHostMount,
			launchr.MustAbs(act.Dir()) + ":" + containerActionMount,
		},
		WorkingDir: containerHostMount,
		Env:        runConf.Env,
		User:       getCurrentUser(),
	}

	errImgEns := errors.New("image ensure error")
	errCreate := errors.New("container create error")
	errStart := errors.New("start error")
	errExecError := launchr.NewExitError(2, fmt.Sprintf("action %q finished with exit code 2", act.ID))

	const (
		stepImageEnsure = iota
		stepContainerCreate
		stepContainerStart
		stepContainerRemove
	)

	successSteps := []mockCallInfo{
		stepImageEnsure: {
			mockFnImageEnsure,
			1, 1,
			[]any{eqImageOpts{driver.ImageOptions{Name: runConf.Image}}},
			[]any{imgBuild, nil},
			nil,
		},
		stepContainerCreate: {
			mockFnContainerCreate,
			1, 1,
			[]any{opts},
			[]any{cid, nil},
			nil,
		},
		stepContainerStart: {
			mockFnContainerStart,
			1, 1,
			[]any{cid, gomock.Any()},
			[]any{nil, nil, nil},
			func(mock *mockCallInfo) {
				resCh := make(chan int, 1)
				mock.ret[0] = resCh
				resCh <- 0
			},
		},
		stepContainerRemove: {
			mockFnContainerRemove,
			1, 1,
			[]any{cid},
			[]any{nil},
			nil,
		},
	}

	tts := []testCase{
		{
			"successful run",
			successSteps,
			nil,
		},
		{
			"image ensure error",
			[]mockCallInfo{
				{
					mockFnImageEnsure,
					1, 1,
					[]any{gomock.Any()},
					[]any{imgBuild, errImgEns},
					nil,
				},
			},
			errImgEns,
		},
		{
			"container create error",
			append(
				slices.Clone(successSteps[stepImageEnsure:stepContainerCreate]),
				mockCallInfo{
					mockFnContainerCreate,
					1, 1,
					[]any{gomock.Any()},
					[]any{"", errCreate},
					nil,
				}),
			errCreate,
		},
		{
			"container create error - empty container id",
			append(
				slices.Clone(successSteps[stepImageEnsure:stepContainerCreate]),
				mockCallInfo{
					mockFnContainerCreate,
					1, 1,
					[]any{gomock.Any()},
					[]any{"", nil},
					nil,
				}),
			errTestAny{},
		},
		{
			"error start container",
			append(
				slices.Clone(successSteps[stepImageEnsure:stepContainerStart]),
				mockCallInfo{
					mockFnContainerStart,
					1, 1,
					[]any{cid, gomock.Any()},
					[]any{nil, nil, errStart},
					nil,
				},
				successSteps[stepContainerRemove],
			),
			errStart,
		},
		{
			"container return error",
			append(
				slices.Clone(successSteps[stepImageEnsure:stepContainerStart]),
				mockCallInfo{
					mockFnContainerStart,
					1, 1,
					[]any{cid, gomock.Any()},
					[]any{gomock.Any(), nil, nil},
					func(mock *mockCallInfo) {
						resCh := make(chan int, 1)
						mock.ret[0] = resCh
						resCh <- 2
					},
				},
				successSteps[stepContainerRemove],
			),
			errExecError,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl, d, r := prepareContainerTestSuite(t)
			a := act.Clone()
			input := NewInput(a, nil, nil, launchr.NoopStreams())
			input.SetValidated(true)
			err := a.SetInput(input)
			require.NoError(t, err)
			defer ctrl.Finish()
			defer r.Close()
			var prev *gomock.Call
			d.EXPECT().ContainerList(gomock.Any(), gomock.Any()).Return(nil) // @todo test different container names
			for _, step := range tt.steps {
				if step.prepFn != nil {
					step.prepFn(&step)
				}
				prev = callContainerDriverMockFn(d, step, prev)
			}
			ctx := context.Background()
			err = r.Execute(ctx, a)
			assertIsSameError(t, tt.expErr, err)
		})
	}
}

func callContainerDriverMockFn(d *containermock.MockContainerRuntime, step mockCallInfo, prev *gomock.Call) *gomock.Call {
	var call *gomock.Call
	switch step.fn {
	case mockFnImageEnsure:
		call = d.EXPECT().
			ImageEnsure(gomock.Any(), step.args[0]).
			Return(step.ret...)
	case mockFnContainerCreate:
		call = d.EXPECT().
			ContainerCreate(gomock.Any(), step.args[0]).
			Return(step.ret...)
	case mockFnContainerStart:
		call = d.EXPECT().
			ContainerStart(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
	case mockFnImageRemove:
		call = d.EXPECT().
			ImageRemove(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
	case mockFnContainerRemove:
		call = d.EXPECT().
			ContainerRemove(gomock.Any(), step.args[0]).
			Return(step.ret...)
	default:
		panic("unknown function: " + step.fn)
	}
	if step.minTimes > 1 {
		call.MinTimes(step.minTimes)
	}
	if step.maxTimes > 1 {
		call.MaxTimes(step.maxTimes)
	}
	if step.maxTimes < 0 {
		call.AnyTimes()
	}

	switch step.minTimes {
	case -1:
		call.AnyTimes()
	default:
	}
	if call != nil && prev != nil {
		call.After(prev)
	}
	return call
}

type fsmy map[string]string

func (f fsmy) MapFS() fstest.MapFS {
	m := make(fstest.MapFS)
	for k, v := range f {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return m
}

func Test_ConfigImageBuildInfo(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		fs     fsmy
		expImg bool
	}

	tts := []testCase{
		{"valid config", fsmy{"config.yaml": validImgsYaml}, true},
		{"no config", fsmy{}, false},
		{"empty config", fsmy{"config.yaml": ""}, false},
		{"invalid config", fsmy{"config.yaml": invalidImgsYaml}, false},
	}
	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := launchr.ConfigFromFS(tt.fs.MapFS())
			cfgImgRes := LaunchrConfigImageBuildResolver{cfg}
			assert.NotNil(t, cfg)
			if img := cfgImgRes.ImageBuildInfo("my/image:version"); (img == nil) == tt.expImg {
				t.Errorf("expected image to find in config directory")
			}
		})
	}
}

const cfgYaml = `
images:
  build:config: ./config
`

const validImgsYaml = `
images:
  my/image:version:
    context: ./
    buildfile: test1.Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - my/image:version2
      - my/image:version3
  my/image2:version:
    context: ./
    buildfile: test2.Dockerfile
    args:
      arg1: val1
      arg2: val2
`

const invalidImgsYaml = `
images:
  - context: ./
    buildfile: test1.Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - my/image:version2
      - my/image:version3
  - context: ./
    buildfile: test2.Dockerfile
    args:
      arg1: val1
      arg2: val2
  - ./
`
