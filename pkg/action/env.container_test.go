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

	"github.com/golang/mock/gomock"
	"github.com/moby/moby/pkg/stdcopy"
	"github.com/stretchr/testify/assert"

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
	r.SetContainerNamePrefix(containerNamePrefix)

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
			imgFn(types.ImagePull, "pulling image", nil),
		},
		{
			"image build local",
			aconf,
			actLoc.ImageBuildInfo(aconf.Image),
			imgFn(types.ImageBuild, "building image (local config)", nil),
		},
		{
			"image build config",
			&DefAction{Image: "build:config"},
			cfgImgRes.ImageBuildInfo("build:config"),
			imgFn(types.ImageBuild, "building image (from config)", nil),
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
			err = act.EnsureLoaded()
			assert.NoError(err)
			a := act.ActionDef()
			imgOpts := types.ImageOptions{Name: a.Image, Build: tt.expBuild}
			d.EXPECT().
				ImageEnsure(ctx, eqImageOpts{imgOpts}).
				Return(tt.ret...)
			err = r.imageEnsure(ctx, act)
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
	eqCfg.Mounts = map[string]string{
		".":     containerHostMount,
		a.Dir(): containerActionMount,
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

	// Image ensure fail.
	cid, err := r.containerCreate(ctx, a, runCfg)
	assert.NoError(err)
	assert.Equal(expCid, cid)

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
	streams := cli.InMemoryStreams()
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
		AttachStdin:  opts.AttachStdin,
		AttachStdout: opts.AttachStdout,
		AttachStderr: opts.AttachStderr,
		Tty:          opts.Tty,
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

func Test_ContainerExec(t *testing.T) {
	t.Parallel()

	cid := "cid"
	act := testContainerAction(nil)
	assert.NoError(t, act.EnsureLoaded())
	actConf := act.ActionDef()
	imgBuild := &types.ImageStatusResponse{Status: types.ImageExists}
	cio := testContainerIO()

	type testCase struct {
		name     string
		prepFn   func(resCh chan types.ContainerWaitResponse, errCh chan error)
		stepArgs [][]interface{}
		stepRet  [][]interface{}
		expErr   error
	}

	opts := types.ContainerCreateOptions{
		ContainerName: genContainerName(act, containerNamePrefix, nil),
		Cmd:           actConf.Command,
		Image:         actConf.Image,
		ExtraHosts:    actConf.ExtraHosts,
		Mounts: map[string]string{
			".":       containerHostMount,
			act.Dir(): containerActionMount,
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
		AttachStdin:  opts.AttachStdin,
		AttachStdout: opts.AttachStdout,
		AttachStderr: opts.AttachStderr,
		Tty:          opts.Tty,
	}

	errImgEns := errors.New("image ensure error")
	errCreate := errors.New("container create error")
	errAny := errors.New("any")
	errAttach := errors.New("attach error")
	errStart := errors.New("start error")
	tts := []testCase{
		{
			"successful run",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{types.ImageOptions{Name: actConf.Image}}}, // ImageEnsure
				{opts},         // ContainerCreate
				{cid, attOpts}, // ContainerAttach
				{cid, types.ContainerWaitOptions{Condition: types.WaitConditionRemoved}}, // ContainerWait
				{cid, types.ContainerStartOptions{}},                                     // ContainerStart
			},
			[][]interface{}{
				{imgBuild, nil}, // ImageEnsure
				{cid, nil},      // ContainerCreate
				{cio, nil},      // ContainerAttach
				{},              // ContainerWait
				{nil},           // ContainerStart
			},
			nil,
		},
		{
			"image ensure error",
			nil,
			[][]interface{}{
				{gomock.Any()}, // ImageEnsure
			},
			[][]interface{}{
				{imgBuild, errImgEns}, // ImageEnsure
			},
			errImgEns,
		},
		{
			"container create error",
			nil,
			[][]interface{}{
				{gomock.Any()}, // ImageEnsure
				{gomock.Any()}, // ContainerCreate
			},
			[][]interface{}{
				{imgBuild, nil}, // ImageEnsure
				{"", errCreate}, // ContainerCreate
			},
			errCreate,
		},
		{
			"container create error - empty container id",
			nil,
			[][]interface{}{
				{gomock.Any()}, // ImageEnsure
				{gomock.Any()}, // ContainerCreate
			},
			[][]interface{}{
				{imgBuild, nil}, // ImageEnsure
				{"", nil},       // ContainerCreate
			},
			errAny,
		},
		{
			"error on container attach",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{types.ImageOptions{Name: actConf.Image}}}, // ImageEnsure
				{gomock.Any()},      // ContainerCreate
				{cid, gomock.Any()}, // ContainerAttach
			},
			[][]interface{}{
				{imgBuild, nil},  // ImageEnsure
				{cid, nil},       // ContainerCreate
				{cio, errAttach}, // ContainerAttach
			},
			errAttach,
		},
		{
			"error start container",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{types.ImageOptions{Name: actConf.Image}}}, // ImageEnsure
				{gomock.Any()},      // ContainerCreate
				{cid, gomock.Any()}, // ContainerAttach
				{cid, gomock.Any()}, // ContainerWait
				{cid, gomock.Any()}, // ContainerStart
			},
			[][]interface{}{
				{imgBuild, nil}, // ImageEnsure
				{cid, nil},      // ContainerCreate
				{cio, nil},      // ContainerAttach
				{},              // ContainerWait
				{errStart},      // ContainerStart
			},
			errStart,
		},
		{
			"container return error",
			func(resCh chan types.ContainerWaitResponse, errCh chan error) {
				resCh <- types.ContainerWaitResponse{StatusCode: 2}
			},
			[][]interface{}{
				{eqImageOpts{types.ImageOptions{Name: actConf.Image}}}, // ImageEnsure
				{gomock.Any()},      // ContainerCreate
				{cid, gomock.Any()}, // ContainerAttach
				{cid, gomock.Any()}, // ContainerWait
				{cid, gomock.Any()}, // ContainerStart
			},
			[][]interface{}{
				{imgBuild, nil}, // ImageEnsure
				{cid, nil},      // ContainerCreate
				{cio, nil},      // ContainerAttach
				{},              // ContainerWait
				{nil},           // ContainerStart
			},
			nil,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resCh, errCh := make(chan types.ContainerWaitResponse, 1), make(chan error, 1)
			assert, ctrl, d, r := prepareContainerTestSuite(t)
			a := act.Clone()
			err := a.SetInput(Input{nil, nil, cli.InMemoryStreams()})
			assert.NoError(err)
			defer ctrl.Finish()
			defer r.Close()
			var prev *gomock.Call
			d.EXPECT().ContainerList(gomock.Any(), gomock.Any()).Return(nil) // @todo test different container names
			for i, args := range tt.stepArgs {
				ret := tt.stepRet[i]
				switch i {
				case 0:
					prev = d.EXPECT().
						ImageEnsure(gomock.Any(), args[0]).
						Return(ret...)
				case 1:
					prev = d.EXPECT().
						ContainerCreate(gomock.Any(), args[0]).
						Return(ret...).
						After(prev)
				case 2:
					prev = d.EXPECT().
						ContainerAttach(gomock.Any(), args[0], args[1]).
						Return(ret...).
						After(prev)
				case 3:
					prev = d.EXPECT().
						ContainerWait(gomock.Any(), args[0], args[1]).
						Return(resCh, errCh).
						After(prev)
				case 4:
					prev = d.EXPECT().
						ContainerStart(gomock.Any(), args[0], args[1]).
						Return(ret...).
						After(prev)
				}
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
