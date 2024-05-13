package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/moby/moby/pkg/jsonmessage"
	"github.com/moby/moby/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	mockdriver "github.com/launchrctl/launchr/pkg/driver/mocks"
	"github.com/launchrctl/launchr/pkg/types"
)

const containerNamePrefix = "test_prefix_"

type eqImageOpts struct {
	x types.ImageOptions
}

func (e eqImageOpts) Matches(x interface{}) bool {
	return assert.ObjectsAreEqual(e.x, x.(types.ImageOptions))
}

func (e eqImageOpts) String() string {
	return fmt.Sprintf("is equal to %v (%T)", e.x, e.x)
}

var cfgImgRes = LaunchrConfigImageBuildResolver{launchrCfg()}

func launchrCfg() launchr.Config {
	cfgRoot := fstest.MapFS{"config.yaml": &fstest.MapFile{Data: []byte(cfgYaml)}}
	return launchr.ConfigFromFS(cfgRoot)
}

func prepareContainerTestSuite(t *testing.T) (*assert.Assertions, *gomock.Controller, *mockdriver.MockContainerRunner, *containerEnv) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	d := mockdriver.NewMockContainerRunner(ctrl)
	d.EXPECT().Close()
	r := &containerEnv{driver: d, dtype: "mock"}
	r.AddImageBuildResolver(cfgImgRes)
	r.SetContainerNameProvider(ContainerNameProvider{Prefix: containerNamePrefix})

	return assert, ctrl, d, r
}

func testContainerAction(aconf *DefAction) *Action {
	if aconf == nil {
		aconf = &DefAction{
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
	return &Action{
		ID:     "test",
		Loader: &Definition{Action: aconf},
		fpath:  "my/action/test/action.yaml",
		wd:     absPath("test"),
	}
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

	actLoc := testContainerAction(&DefAction{
		Image: "build:local",
		Build: &types.BuildDefinition{
			Context: ".",
		},
	})
	err := actLoc.EnsureLoaded()
	assert.NoError(t, err)
	type testCase struct {
		name     string
		action   *DefAction
		expBuild *types.BuildDefinition
		ret      []interface{}
	}

	imgFn := func(s types.ImageStatus, pstr string, err error) []interface{} {
		var p io.ReadCloser
		if pstr != "" {
			p = io.NopCloser(strings.NewReader(pstr))
		}
		var r *types.ImageStatusResponse
		if s != -1 {
			r = &types.ImageStatusResponse{Status: s, Progress: p}
		}
		return []interface{}{r, err}
	}

	aconf := actLoc.ActionDef()
	tts := []testCase{
		{
			"image exists",
			&DefAction{Image: "exists"},
			nil,
			imgFn(types.ImageExists, "", nil),
		},
		{
			"image pulled",
			&DefAction{Image: "pull"},
			nil,
			imgFn(types.ImagePull, `{"stream":"Successfully pulled image\n"}`, nil),
		},
		{
			"image pulled error",
			&DefAction{Image: "pull"},
			nil,
			imgFn(
				types.ImagePull,
				`{"errorDetail":{"code":1,"message":"fake pull error"},"error":"fake pull error"}`,
				&jsonmessage.JSONError{Code: 1, Message: "fake pull error"},
			),
		},
		{
			"image build local",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(types.ImageBuild, `{"stream":"Successfully built image \"local\"\n"}`, nil),
		},
		{
			"image build local error",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(
				types.ImageBuild,
				`{"errorDetail":{"code":1,"message":"fake build error"},"error":"fake build error"}`,
				&jsonmessage.JSONError{Code: 1, Message: "fake build error"},
			),
		},
		{
			"image build config",
			&DefAction{Image: "build:config"},
			cfgImgRes.ImageBuildInfo("build:config"),
			imgFn(types.ImageBuild, `{"stream":"Successfully built image \"config\"\n"}`, nil),
		},
		{
			"driver error",
			&DefAction{Image: ""},
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
			act.input = Input{
				IO: cli.NoopStreams(),
			}
			err = act.EnsureLoaded()
			assert.NoError(err)
			a := act.ActionDef()
			imgOpts := types.ImageOptions{Name: a.Image, Build: tt.expBuild}
			d.EXPECT().
				ImageEnsure(ctx, eqImageOpts{imgOpts}).
				Return(tt.ret...)
			err = r.imageEnsure(ctx, act)
			assert.Equal(tt.ret[1], err)
		})
	}
}

func Test_ContainerExec_imageRemove(t *testing.T) {
	t.Parallel()

	actLoc := testContainerAction(&DefAction{
		Image: "build:local",
		Build: &types.BuildDefinition{
			Context: ".",
		},
	})
	err := actLoc.EnsureLoaded()
	assert.NoError(t, err)
	type testCase struct {
		name     string
		action   *DefAction
		expBuild *types.BuildDefinition
		ret      []interface{}
	}

	tts := []testCase{
		{
			"image removed",
			actLoc.ActionDef(),
			nil,
			[]interface{}{&types.ImageRemoveResponse{Status: types.ImageRemoved}, nil},
		},
		{
			"failed to remove",
			&DefAction{Image: "failed"},
			nil,
			[]interface{}{nil, fmt.Errorf("failed to remove")},
		},
	}

	for _, tt := range tts {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert, ctrl, d, r := prepareContainerTestSuite(t)
			ctx := context.Background()

			defer ctrl.Finish()
			defer r.driver.Close()

			act := testContainerAction(tt.action)
			act.input = Input{
				IO: cli.NoopStreams(),
			}

			err := act.EnsureLoaded()
			assert.NoError(err)

			a := act.ActionDef()
			imgOpts := types.ImageRemoveOptions{Force: true, PruneChildren: false}
			d.EXPECT().
				ImageRemove(ctx, a.Image, gomock.Eq(imgOpts)).
				Return(tt.ret...)
			err = r.imageRemove(ctx, act)

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
	assert.NoError(a.EnsureLoaded())
	act := a.ActionDef()

	runCfg := &types.ContainerCreateOptions{
		ContainerName: "container",
		NetworkMode:   types.NetworkModeHost,
		ExtraHosts:    act.ExtraHosts,
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
		absPath(a.WorkDir()) + ":" + containerHostMount,
		absPath(a.Dir()) + ":" + containerActionMount,
	}
	eqCfg.WorkingDir = containerHostMount
	eqCfg.Cmd = act.Command
	eqCfg.Image = act.Image

	ctx := context.Background()

	// Normal create.
	expCid := "container_id"
	d.EXPECT().
		ImageEnsure(ctx, types.ImageOptions{Name: act.Image}).
		Return(&types.ImageStatusResponse{Status: types.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err := r.containerCreate(ctx, a, runCfg)
	assert.NoError(err)
	assert.Equal(expCid, cid)

	// Create with a custom wd
	a.def.WD = "../myactiondir"
	wd := absPath(a.def.WD)
	eqCfg.Binds = []string{
		wd + ":" + containerHostMount,
		absPath(a.Dir()) + ":" + containerActionMount,
	}
	d.EXPECT().
		ImageEnsure(ctx, types.ImageOptions{Name: act.Image}).
		Return(&types.ImageStatusResponse{Status: types.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err = r.containerCreate(ctx, a, runCfg)
	assert.NoError(err)
	assert.Equal(expCid, cid)

	// Create with anonymous volumes.
	r.useVolWD = true
	eqCfg.Binds = nil
	eqCfg.Volumes = map[string]struct{}{
		containerHostMount:   {},
		containerActionMount: {},
	}
	d.EXPECT().
		ImageEnsure(ctx, types.ImageOptions{Name: act.Image}).
		Return(&types.ImageStatusResponse{Status: types.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	cid, err = r.containerCreate(ctx, a, runCfg)
	assert.NoError(err)
	assert.Equal(expCid, cid)

	// Image ensure fail.
	errImg := fmt.Errorf("error on image ensure")
	d.EXPECT().
		ImageEnsure(ctx, types.ImageOptions{Name: act.Image}).
		Return(nil, errImg)

	cid, err = r.containerCreate(ctx, a, runCfg)
	assert.Error(err)
	assert.Equal("", cid)

	// Container create fail.
	expErr := fmt.Errorf("driver container create error")
	d.EXPECT().
		ImageEnsure(ctx, types.ImageOptions{Name: act.Image}).
		Return(&types.ImageStatusResponse{Status: types.ImageExists}, nil)
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
		chanFn    func(resCh chan types.ContainerWaitResponse, errCh chan error)
		waitCond  types.WaitCondition
		expStatus int
	}

	tts := []testCase{
		{
			"condition removed",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			types.WaitConditionRemoved,
			0,
		},
		{
			"condition next exit",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			types.WaitConditionNextExit,
			0,
		},
		{
			"return exit code",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 2}
			},
			types.WaitConditionRemoved,
			2,
		},
		{
			"fail on container run",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0, Error: errors.New("fail run")}
			},
			types.WaitConditionRemoved,
			125,
		},
		{
			"fail on wait",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				errCh <- errors.New("fail wait")
			},
			types.WaitConditionRemoved,
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
			resCh, errCh := make(chan types.ContainerWaitResponse, 1), make(chan error, 1)
			tt.chanFn(resCh, errCh)
			d.EXPECT().
				ContainerWait(ctx, cid, types.ContainerWaitOptions{Condition: tt.waitCond}).
				Return(resCh, errCh)

			// Test waiting and status.
			autoRemove := false
			if tt.waitCond == types.WaitConditionRemoved {
				autoRemove = true
			}
			runCfg := &types.ContainerCreateOptions{AutoRemove: autoRemove}
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
	streams := cli.NoopStreams()
	defer ctrl.Finish()
	defer r.Close()

	ctx := context.Background()
	cid := ""
	cio := testContainerIO()
	opts := &types.ContainerCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	attOpts := types.ContainerAttachOptions{
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
	assert.NoError(err)
	assert.NoError(<-errCh)
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
	args     []interface{}
	ret      []interface{}
}

func Test_ContainerExec(t *testing.T) {
	t.Parallel()

	cid := "cid"
	act := testContainerAction(nil)
	assert.NoError(t, act.EnsureLoaded())
	actConf := act.ActionDef()
	imgBuild := &types.ImageStatusResponse{Status: types.ImageExists}
	cio := testContainerIO()
	nprv := ContainerNameProvider{Prefix: containerNamePrefix}

	type testCase struct {
		name   string
		prepFn func(resCh chan types.ContainerWaitResponse, errCh chan error)
		steps  []mockCallInfo
		expErr error
	}

	opts := types.ContainerCreateOptions{
		ContainerName: nprv.Get(act.ID),
		Cmd:           actConf.Command,
		Image:         actConf.Image,
		NetworkMode:   types.NetworkModeHost,
		ExtraHosts:    actConf.ExtraHosts,
		Binds: []string{
			absPath(act.WorkDir()) + ":" + containerHostMount,
			absPath(act.Dir()) + ":" + containerActionMount,
		},
		WorkingDir:   containerHostMount,
		AutoRemove:   true,
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Env:          actConf.Env,
		User:         getCurrentUser(),
	}
	attOpts := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  opts.AttachStdin,
		Stdout: opts.AttachStdout,
		Stderr: opts.AttachStderr,
	}

	errImgEns := errors.New("image ensure error")
	errCreate := errors.New("container create error")
	errAny := errors.New("any")
	errAttach := errors.New("attach error")
	errStart := errors.New("start error")
	errExecError := RunStatusError{code: 2, msg: "action \"test\" finished with the exit code 2"}

	successSteps := []mockCallInfo{
		{
			"ImageEnsure",
			1, 1,
			[]interface{}{eqImageOpts{types.ImageOptions{Name: actConf.Image}}},
			[]interface{}{imgBuild, nil},
		},
		{
			"ContainerCreate",
			1, 1,
			[]interface{}{opts},
			[]interface{}{cid, nil},
		},
		{
			"ContainerAttach",
			1, 1,
			[]interface{}{cid, attOpts},
			[]interface{}{cio, nil},
		},
		{
			"ContainerWait",
			1, 1,
			[]interface{}{cid, types.ContainerWaitOptions{Condition: types.WaitConditionRemoved}},
			[]interface{}{},
		},
		{
			"ContainerStart",
			1, 1,
			[]interface{}{cid, types.ContainerStartOptions{}},
			[]interface{}{nil},
		},
	}

	tts := []testCase{
		{
			"successful run",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
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
					[]interface{}{gomock.Any()},
					[]interface{}{imgBuild, errImgEns},
				},
			},
			errImgEns,
		},
		{
			"container create error",
			nil,
			append(
				copySlice(successSteps[0:1]),
				mockCallInfo{
					"ContainerCreate",
					1, 1,
					[]interface{}{gomock.Any()},
					[]interface{}{"", errCreate},
				}),
			errCreate,
		},
		{
			"container create error - empty container id",
			nil,
			append(
				copySlice(successSteps[0:1]),
				mockCallInfo{
					"ContainerCreate",
					1, 1,
					[]interface{}{gomock.Any()},
					[]interface{}{"", nil},
				}),
			errAny,
		},
		{
			"error on container attach",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			append(
				copySlice(successSteps[0:2]),
				mockCallInfo{
					"ContainerAttach",
					1, 1,
					[]interface{}{cid, gomock.Any()},
					[]interface{}{cio, errAttach},
				},
			),
			errAttach,
		},
		{
			"error start container",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			append(
				copySlice(successSteps[0:4]),
				mockCallInfo{
					"ContainerStart",
					1, 1,
					[]interface{}{cid, gomock.Any()},
					[]interface{}{errStart},
				},
			),
			errStart,
		},
		{
			"container return error",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 2}
			},
			append(
				copySlice(successSteps[0:4]),
				mockCallInfo{
					"ContainerStart",
					1, 1,
					[]interface{}{cid, gomock.Any()},
					[]interface{}{nil},
				},
			),
			errExecError,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resCh, errCh := make(chan types.ContainerWaitResponse, 1), make(chan error, 1)
			assert, ctrl, d, r := prepareContainerTestSuite(t)
			a := act.Clone()
			err := a.SetInput(Input{nil, nil, cli.NoopStreams(), nil})
			assert.NoError(err)
			defer ctrl.Finish()
			defer r.Close()
			var prev *gomock.Call
			d.EXPECT().ContainerList(gomock.Any(), gomock.Any()).Return(nil) // @todo test different container names
			for _, step := range tt.steps {
				if step.fn == "ContainerWait" { //nolint:goconst
					step.ret = []interface{}{resCh, errCh}
				}
				prev = callContainerDriverMockFn(d, step, prev)
			}
			if tt.prepFn != nil {
				tt.prepFn(resCh, errCh)
			}
			ctx := context.Background()
			err = r.Execute(ctx, a)
			if tt.expErr != errAny {
				assert.Equal(tt.expErr, err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func copySlice[T any](arr []T) []T {
	c := make([]T, len(arr))
	copy(c, arr)
	return c
}

func callContainerDriverMockFn(d *mockdriver.MockContainerRunner, step mockCallInfo, prev *gomock.Call) *gomock.Call {
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
  my/image3:version: ./
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
