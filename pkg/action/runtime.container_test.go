package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/driver"
	containermock "github.com/launchrctl/launchr/pkg/driver/mocks"
)

const containerNamePrefix = "test_prefix_"

type eqImageOpts struct {
	x driver.ImageOptions
}

func (e eqImageOpts) Matches(x any) bool {
	return assert.ObjectsAreEqual(e.x, x.(driver.ImageOptions))
}

func (e eqImageOpts) String() string {
	return fmt.Sprintf("is equal to %v (%T)", e.x, e.x)
}

var cfgImgRes = LaunchrConfigImageBuildResolver{launchrCfg()}

func launchrCfg() launchr.Config {
	cfgRoot := fstest.MapFS{"config.yaml": &fstest.MapFile{Data: []byte(cfgYaml)}}
	return launchr.ConfigFromFS(cfgRoot)
}

func prepareContainerTestSuite(t *testing.T) (*assert.Assertions, *gomock.Controller, *containermock.MockContainerRuntime, *runtimeContainer) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	d := containermock.NewMockContainerRuntime(ctrl)
	d.EXPECT().Close()
	r := &runtimeContainer{crt: d, rtype: "mock"}
	r.AddImageBuildResolver(cfgImgRes)
	r.SetContainerNameProvider(ContainerNameProvider{Prefix: containerNamePrefix})

	return assert, ctrl, d, r
}

func testContainerAction(cdef *DefRuntimeContainer) *Action {
	if cdef == nil {
		cdef = &DefRuntimeContainer{
			Image: "myimage",
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

func testContainerIO() *driver.ContainerInOut {
	outBytes := []byte("0test stdOut")
	// Set row header for moby.stdCopy proper parsing of combined streams.
	outBytes[0] = byte(stdcopy.Stdout)
	return &driver.ContainerInOut{
		In: &fakeWriter{
			Buffer: bytes.NewBuffer([]byte{}),
		},
		Out: bytes.NewBuffer(outBytes),
	}
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
			r = &driver.ImageStatusResponse{Status: s, Progress: p}
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
			imgFn(driver.ImagePull, `{"stream":"Successfully pulled image\n"}`, nil),
		},
		{
			"image pulled error",
			&DefRuntimeContainer{Image: "pull"},
			nil,
			imgFn(
				driver.ImagePull,
				`{"errorDetail":{"code":1,"message":"fake pull error"},"error":"fake pull error"}`,
				&jsonmessage.JSONError{Code: 1, Message: "fake pull error"},
			),
		},
		{
			"image build local",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(driver.ImageBuild, `{"stream":"Successfully built image \"local\"\n"}`, nil),
		},
		{
			"image build local error",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(
				driver.ImageBuild,
				`{"errorDetail":{"code":1,"message":"fake build error"},"error":"fake build error"}`,
				&jsonmessage.JSONError{Code: 1, Message: "fake build error"},
			),
		},
		{
			"image build config",
			&DefRuntimeContainer{Image: "build:config"},
			cfgImgRes.ImageBuildInfo("build:config"),
			imgFn(driver.ImageBuild, `{"stream":"Successfully built image \"config\"\n"}`, nil),
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
			assert, ctrl, d, r := prepareContainerTestSuite(t)
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
			assert.Equal(tt.ret[1], err)
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
			assert, ctrl, d, r := prepareContainerTestSuite(t)
			ctx := context.Background()

			defer ctrl.Finish()
			defer r.crt.Close()

			act := testContainerAction(tt.action)
			act.input = NewInput(act, nil, nil, launchr.NoopStreams())

			run := act.RuntimeDef().Container
			imgOpts := driver.ImageRemoveOptions{Force: true, PruneChildren: false}
			d.EXPECT().
				ImageRemove(ctx, run.Image, gomock.Eq(imgOpts)).
				Return(tt.ret...)
			err := r.imageRemove(ctx, act)

			assert.Equal(err, tt.ret[1])
		})
	}
}

func Test_ContainerExec_containerCreate(t *testing.T) {
	t.Parallel()
	assert, ctrl, d, r := prepareContainerTestSuite(t)
	defer ctrl.Finish()
	defer r.Close()

	a := testContainerAction(nil)
	run := a.RuntimeDef()

	runCfg := &driver.ContainerCreateOptions{
		ContainerName: "container",
		NetworkMode:   driver.NetworkModeHost,
		ExtraHosts:    run.Container.ExtraHosts,
		AutoRemove:    true,
		OpenStdin:     true,
		StdinOnce:     true,
		AttachStdin:   true,
		AttachStdout:  true,
		AttachStderr:  true,
		Tty:           true,
		Env: []string{
			"env1=val1",
			"env2=val2",
		},
	}

	eqCfg := *runCfg
	eqCfg.Binds = []string{
		launchr.MustAbs(a.WorkDir()) + ":" + containerHostMount,
		launchr.MustAbs(a.Dir()) + ":" + containerActionMount,
	}
	eqCfg.WorkingDir = containerHostMount
	eqCfg.Cmd = run.Container.Command
	eqCfg.Image = run.Container.Image

	ctx := context.Background()

	// Normal create.
	expCid := "container_id"
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: run.Container.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err := r.containerCreate(ctx, a, runCfg)
	require.NoError(t, err)
	assert.Equal(expCid, cid)

	// Create with a custom wd
	a.def.WD = "../myactiondir"
	wd := launchr.MustAbs(a.def.WD)
	eqCfg.Binds = []string{
		wd + ":" + containerHostMount,
		launchr.MustAbs(a.Dir()) + ":" + containerActionMount,
	}
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: run.Container.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err = r.containerCreate(ctx, a, runCfg)
	require.NoError(t, err)
	assert.Equal(expCid, cid)

	// Create with anonymous volumes.
	r.useVolWD = true
	eqCfg.Binds = nil
	eqCfg.Volumes = map[string]struct{}{
		containerHostMount:   {},
		containerActionMount: {},
	}
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: run.Container.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err = r.containerCreate(ctx, a, runCfg)
	require.NoError(t, err)
	assert.Equal(expCid, cid)

	// Image ensure fail.
	errImg := fmt.Errorf("error on image ensure")
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: run.Container.Image}).
		Return(nil, errImg)

	cid, err = r.containerCreate(ctx, a, runCfg)
	assert.Error(err)
	assert.Equal("", cid)

	// Container create fail.
	expErr := fmt.Errorf("container create error")
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: run.Container.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Any()).
		Return("", expErr)
	cid, err = r.containerCreate(ctx, a, runCfg)
	assert.Error(err)
	assert.Equal("", cid)
}

func Test_ContainerExec_containerWait(t *testing.T) {
	t.Parallel()
	assert, ctrl, d, r := prepareContainerTestSuite(t)
	defer ctrl.Finish()
	defer r.Close()

	type testCase struct {
		name      string
		chanFn    func(resCh chan driver.ContainerWaitResponse, errCh chan error)
		waitCond  driver.WaitCondition
		expStatus int
	}

	tts := []testCase{
		{
			"condition removed",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			driver.WaitConditionRemoved,
			0,
		},
		{
			"condition next exit",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			driver.WaitConditionNextExit,
			0,
		},
		{
			"return exit code",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 2}
			},
			driver.WaitConditionRemoved,
			2,
		},
		{
			"fail on container run",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0, Error: errors.New("fail run")}
			},
			driver.WaitConditionRemoved,
			125,
		},
		{
			"fail on wait",
			func(_ chan driver.ContainerWaitResponse, errCh chan error) {
				errCh <- errors.New("fail wait")
			},
			driver.WaitConditionRemoved,
			125,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Set timeout for broken channel cases.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Prepare channels with buffer for non-blocking.
			cid := ""
			resCh, errCh := make(chan driver.ContainerWaitResponse, 1), make(chan error, 1)
			tt.chanFn(resCh, errCh)
			d.EXPECT().
				ContainerWait(ctx, cid, driver.ContainerWaitOptions{Condition: tt.waitCond}).
				Return(resCh, errCh)

			// Test waiting and status.
			autoRemove := false
			if tt.waitCond == driver.WaitConditionRemoved {
				autoRemove = true
			}
			runCfg := &driver.ContainerCreateOptions{AutoRemove: autoRemove}
			ch := r.containerWait(ctx, cid, runCfg)
			assert.Equal(tt.expStatus, <-ch)
		})
	}
}

type fakeWriter struct {
	*bytes.Buffer
}

func (f *fakeWriter) Close() error {
	f.Buffer.Reset()
	return nil
}

func Test_ContainerExec_containerAttach(t *testing.T) {
	t.Parallel()
	assert, ctrl, d, r := prepareContainerTestSuite(t)
	streams := launchr.NoopStreams()
	defer ctrl.Finish()
	defer r.Close()

	ctx := context.Background()
	cid := ""
	cio := testContainerIO()
	opts := &driver.ContainerCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	attOpts := driver.ContainerAttachOptions{
		Stream: true,
		Stdin:  opts.AttachStdin,
		Stdout: opts.AttachStdout,
		Stderr: opts.AttachStderr,
	}
	d.EXPECT().
		ContainerAttach(ctx, cid, attOpts).
		Return(cio, nil)
	acio, errCh, err := r.attachContainer(ctx, streams, cid, opts)
	assert.Equal(acio, cio)
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	_ = acio.Close()

	expErr := errors.New("fail to attach")
	d.EXPECT().
		ContainerAttach(ctx, cid, attOpts).
		Return(nil, expErr)
	acio, errCh, err = r.attachContainer(ctx, streams, cid, opts)
	assert.Equal(nil, acio)
	assert.Equal(expErr, err)
	assert.Nil(errCh)
}

type mockCallInfo struct {
	fn       string
	minTimes int
	maxTimes int
	args     []any
	ret      []any
}

func Test_ContainerExec(t *testing.T) {
	t.Parallel()

	cid := "cid"
	act := testContainerAction(nil)
	runConf := act.RuntimeDef().Container
	imgBuild := &driver.ImageStatusResponse{Status: driver.ImageExists}
	cio := testContainerIO()
	nprv := ContainerNameProvider{Prefix: containerNamePrefix}

	type testCase struct {
		name   string
		prepFn func(resCh chan driver.ContainerWaitResponse, errCh chan error)
		steps  []mockCallInfo
		expErr error
	}

	opts := driver.ContainerCreateOptions{
		ContainerName: nprv.Get(act.ID),
		Cmd:           runConf.Command,
		Image:         runConf.Image,
		NetworkMode:   driver.NetworkModeHost,
		ExtraHosts:    runConf.ExtraHosts,
		Binds: []string{
			launchr.MustAbs(act.WorkDir()) + ":" + containerHostMount,
			launchr.MustAbs(act.Dir()) + ":" + containerActionMount,
		},
		WorkingDir:   containerHostMount,
		AutoRemove:   true,
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Env:          runConf.Env,
		User:         getCurrentUser(),
	}
	attOpts := driver.ContainerAttachOptions{
		Stream: true,
		Stdin:  opts.AttachStdin,
		Stdout: opts.AttachStdout,
		Stderr: opts.AttachStderr,
	}

	errImgEns := errors.New("image ensure error")
	errCreate := errors.New("container create error")
	errAttach := errors.New("attach error")
	errStart := errors.New("start error")
	errExecError := launchr.NewExitError(2, fmt.Sprintf("action %q finished with exit code 2", act.ID))

	successSteps := []mockCallInfo{
		{
			"ImageEnsure",
			1, 1,
			[]any{eqImageOpts{driver.ImageOptions{Name: runConf.Image}}},
			[]any{imgBuild, nil},
		},
		{
			"ContainerCreate",
			1, 1,
			[]any{opts},
			[]any{cid, nil},
		},
		{
			"ContainerAttach",
			1, 1,
			[]any{cid, attOpts},
			[]any{cio, nil},
		},
		{
			"ContainerWait",
			1, 1,
			[]any{cid, driver.ContainerWaitOptions{Condition: driver.WaitConditionRemoved}},
			[]any{},
		},
		{
			"ContainerStart",
			1, 1,
			[]any{cid, driver.ContainerStartOptions{}},
			[]any{nil},
		},
	}

	tts := []testCase{
		{
			"successful run",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			successSteps,
			nil,
		},
		{
			"image ensure error",
			nil,
			[]mockCallInfo{
				{
					"ImageEnsure",
					1, 1,
					[]any{gomock.Any()},
					[]any{imgBuild, errImgEns},
				},
			},
			errImgEns,
		},
		{
			"container create error",
			nil,
			append(
				slices.Clone(successSteps[0:1]),
				mockCallInfo{
					"ContainerCreate",
					1, 1,
					[]any{gomock.Any()},
					[]any{"", errCreate},
				}),
			errCreate,
		},
		{
			"container create error - empty container id",
			nil,
			append(
				slices.Clone(successSteps[0:1]),
				mockCallInfo{
					"ContainerCreate",
					1, 1,
					[]any{gomock.Any()},
					[]any{"", nil},
				}),
			errTestAny{},
		},
		{
			"error on container attach",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			append(
				slices.Clone(successSteps[0:2]),
				mockCallInfo{
					"ContainerAttach",
					1, 1,
					[]any{cid, gomock.Any()},
					[]any{cio, errAttach},
				},
			),
			errAttach,
		},
		{
			"error start container",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			append(
				slices.Clone(successSteps[0:4]),
				mockCallInfo{
					"ContainerStart",
					1, 1,
					[]any{cid, gomock.Any()},
					[]any{errStart},
				},
			),
			errStart,
		},
		{
			"container return error",
			func(resCh chan driver.ContainerWaitResponse, _ chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 2}
			},
			append(
				slices.Clone(successSteps[0:4]),
				mockCallInfo{
					"ContainerStart",
					1, 1,
					[]any{cid, gomock.Any()},
					[]any{nil},
				},
			),
			errExecError,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resCh, errCh := make(chan driver.ContainerWaitResponse, 1), make(chan error, 1)
			_, ctrl, d, r := prepareContainerTestSuite(t)
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
				if step.fn == "ContainerWait" { //nolint:goconst
					step.ret = []any{resCh, errCh}
				}
				prev = callContainerDriverMockFn(d, step, prev)
			}
			if tt.prepFn != nil {
				tt.prepFn(resCh, errCh)
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
	case "ImageEnsure":
		call = d.EXPECT().
			ImageEnsure(gomock.Any(), step.args[0]).
			Return(step.ret...)
	case "ContainerCreate":
		call = d.EXPECT().
			ContainerCreate(gomock.Any(), step.args[0]).
			Return(step.ret...)
	case "ContainerAttach":
		call = d.EXPECT().
			ContainerAttach(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
	case "ContainerWait":
		call = d.EXPECT().
			ContainerWait(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
	case "ContainerStart":
		call = d.EXPECT().
			ContainerStart(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
	case "ImageRemove":
		call = d.EXPECT().
			ImageRemove(gomock.Any(), step.args[0], step.args[1]).
			Return(step.ret...)
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
	assert := assert.New(t)

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
			assert.NotNil(cfg)
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
