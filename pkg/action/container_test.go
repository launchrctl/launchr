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

	"github.com/launchrctl/launchr/pkg/cli"
	"github.com/launchrctl/launchr/pkg/driver"
	mockdriver "github.com/launchrctl/launchr/pkg/driver/mocks"
)

var gCfgYaml = `
images:
  build:global: ./global
`

type eqImageOpts struct {
	x driver.ImageOptions
}

func (e eqImageOpts) Matches(x interface{}) bool {
	m := assert.ObjectsAreEqual(e.x, x.(driver.ImageOptions))
	return m
}

func (e eqImageOpts) String() string {
	return fmt.Sprintf("is equal to %v (%T)", e.x, e.x)
}

func prepareContainerTestSuite(t *testing.T) (*assert.Assertions, *gomock.Controller, *mockdriver.MockContainerRunner, *containerExec) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	d := mockdriver.NewMockContainerRunner(ctrl)
	d.EXPECT().Close()
	r := &containerExec{d, "mock"}

	return assert, ctrl, d, r
}

func getFakeAppCli() cli.Cli {
	gcfgRoot := fstest.MapFS{
		"config.yaml": &fstest.MapFile{Data: []byte(gCfgYaml)},
	}
	appCli, _ := cli.NewAppCli(
		cli.WithFakeStreams(),
		cli.WithGlobalConfigFromDir(gcfgRoot),
	)
	return appCli
}

func testContainerCmd(a *Action) *Command {
	if a == nil {
		a = &Action{
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
	return &Command{
		CommandName: "test",
		Loader:      &testActionLoader{cfg: &Config{Action: a}},
		Filepath:    "my/action/test/action.yaml",
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

	appCli := getFakeAppCli()
	cmdLocal := testContainerCmd(&Action{
		Image: "build:local",
		Build: &cli.BuildDefinition{
			Context: "./",
		},
	})
	err := cmdLocal.Compile()
	assert.NoError(t, err)
	type testCase struct {
		name     string
		action   *Action
		expBuild *cli.BuildDefinition
		ret      []interface{}
	}

	imgFn := func(s driver.ImageStatus, pstr string, err error) []interface{} {
		var p io.ReadCloser
		if pstr != "" {
			p = io.NopCloser(strings.NewReader(pstr))
		}
		var r *driver.ImageStatusResponse
		if s != -1 {
			r = &driver.ImageStatusResponse{Status: s, Progress: p}
		}
		return []interface{}{r, err}
	}

	a := cmdLocal.Action()
	tts := []testCase{
		{
			"image exists",
			&Action{Image: "exists"},
			nil,
			imgFn(driver.ImageExists, "", nil),
		},
		{
			"image pulled",
			&Action{Image: "pull"},
			nil,
			imgFn(driver.ImagePull, "pulling image", nil),
		},
		{
			"image build local",
			a,
			a.BuildDefinition(cmdLocal.Dir()),
			imgFn(driver.ImageBuild, "building image (local config)", nil),
		},
		{
			"image build global",
			&Action{Image: "build:global"},
			appCli.Config().ImageBuildInfo("build:global"),
			imgFn(driver.ImageBuild, "building image (global config)", nil),
		},
		{
			"driver error",
			&Action{Image: ""},
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
			cmd := testContainerCmd(tt.action)
			err = cmd.Compile()
			assert.NoError(err)
			a := cmd.Action()
			imgOpts := driver.ImageOptions{Name: a.Image, Build: tt.expBuild}
			d.EXPECT().
				ImageEnsure(ctx, eqImageOpts{imgOpts}).
				Return(tt.ret...)
			err = r.imageEnsure(ctx, appCli, cmd)
			assert.Equal(err, tt.ret[1])
		})
	}
}

func Test_ContainerExec_containerCreate(t *testing.T) {
	t.Parallel()
	assert, ctrl, d, r := prepareContainerTestSuite(t)
	appCli := getFakeAppCli()
	defer ctrl.Finish()
	defer r.Close()

	cmd := testContainerCmd(nil)
	assert.NoError(cmd.Compile())
	act := cmd.Action()

	runCfg := &driver.ContainerCreateOptions{
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
		"./":      containerHostMount,
		cmd.Dir(): containerActionMount,
	}
	eqCfg.WorkingDir = containerHostMount
	eqCfg.Cmd = act.Command
	eqCfg.Image = act.Image

	ctx := context.Background()

	// Normal create.
	expCid := "container_id"
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: act.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Eq(eqCfg)).
		Return(expCid, nil)

	// Image ensure fail.
	cid, err := r.containerCreate(ctx, appCli, cmd, runCfg)
	assert.NoError(err)
	assert.Equal(expCid, cid)

	errImg := fmt.Errorf("error on image ensure")
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: act.Image}).
		Return(nil, errImg)

	cid, err = r.containerCreate(ctx, appCli, cmd, runCfg)
	assert.Error(err)
	assert.Equal("", cid)

	// Container create fail.
	expErr := fmt.Errorf("driver container create error")
	d.EXPECT().
		ImageEnsure(ctx, driver.ImageOptions{Name: act.Image}).
		Return(&driver.ImageStatusResponse{Status: driver.ImageExists}, nil)
	d.EXPECT().
		ContainerCreate(ctx, gomock.Any()).
		Return("", expErr)
	cid, err = r.containerCreate(ctx, appCli, cmd, runCfg)
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
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			driver.WaitConditionRemoved,
			0,
		},
		{
			"condition next exit",
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			driver.WaitConditionNextExit,
			0,
		},
		{
			"return exit code",
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 2}
			},
			driver.WaitConditionRemoved,
			2,
		},
		{
			"fail on container run",
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0, Error: errors.New("fail run")}
			},
			driver.WaitConditionRemoved,
			125,
		},
		{
			"fail on wait",
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
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
	appCli := getFakeAppCli()
	defer ctrl.Finish()
	defer r.Close()

	ctx := context.Background()
	cid := ""
	cio := testContainerIO()
	config := &driver.ContainerCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	attOpts := driver.ContainerAttachOptions{
		AttachStdin:  config.AttachStdin,
		AttachStdout: config.AttachStdout,
		AttachStderr: config.AttachStderr,
		Tty:          config.Tty,
	}
	d.EXPECT().
		ContainerAttach(ctx, cid, attOpts).
		Return(cio, nil)
	acio, errCh, err := r.attachContainer(ctx, appCli, cid, config)
	assert.Equal(acio, cio)
	assert.NoError(err)
	assert.NoError(<-errCh)
	_ = acio.Close()

	expErr := errors.New("fail to attach")
	d.EXPECT().
		ContainerAttach(ctx, cid, attOpts).
		Return(nil, expErr)
	acio, errCh, err = r.attachContainer(ctx, appCli, cid, config)
	assert.Equal(nil, acio)
	assert.Equal(expErr, err)
	assert.Nil(errCh)
}

func Test_ContainerExec(t *testing.T) {
	t.Parallel()

	cid := "cid"
	cmd := testContainerCmd(nil)
	assert.NoError(t, cmd.Compile())
	act := cmd.Action()
	imgBuild := &driver.ImageStatusResponse{Status: driver.ImageExists}
	cio := testContainerIO()

	type testCase struct {
		name     string
		prepFn   func(resCh chan driver.ContainerWaitResponse, errCh chan error)
		stepArgs [][]interface{}
		stepRet  [][]interface{}
		expErr   error
	}

	config := driver.ContainerCreateOptions{
		ContainerName: genContainerName(cmd, nil),
		Cmd:           act.Command,
		Image:         act.Image,
		ExtraHosts:    act.ExtraHosts,
		Mounts: map[string]string{
			"./":      containerHostMount,
			cmd.Dir(): containerActionMount,
		},
		WorkingDir:   containerHostMount,
		AutoRemove:   true,
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Env:          act.Env,
	}
	attOpts := driver.ContainerAttachOptions{
		AttachStdin:  config.AttachStdin,
		AttachStdout: config.AttachStdout,
		AttachStderr: config.AttachStderr,
		Tty:          config.Tty,
	}

	errImgEns := errors.New("image ensure error")
	errCreate := errors.New("container create error")
	errAny := errors.New("any")
	errAttach := errors.New("attach error")
	errStart := errors.New("start error")
	tts := []testCase{
		{
			"successful run",
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{driver.ImageOptions{Name: act.Image}}}, // ImageEnsure
				{config},       // ContainerCreate
				{cid, attOpts}, // ContainerAttach
				{cid, driver.ContainerWaitOptions{Condition: driver.WaitConditionRemoved}}, // ContainerWait
				{cid, driver.ContainerStartOptions{}},                                      // ContainerStart
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
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{driver.ImageOptions{Name: act.Image}}}, // ImageEnsure
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
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 0}
			},
			[][]interface{}{
				{eqImageOpts{driver.ImageOptions{Name: act.Image}}}, // ImageEnsure
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
			func(resCh chan driver.ContainerWaitResponse, errCh chan error) {
				resCh <- driver.ContainerWaitResponse{StatusCode: 2}
			},
			[][]interface{}{
				{eqImageOpts{driver.ImageOptions{Name: act.Image}}}, // ImageEnsure
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
			errAny,
		},
	}

	for _, tt := range tts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resCh, errCh := make(chan driver.ContainerWaitResponse, 1), make(chan error, 1)
			assert, ctrl, d, r := prepareContainerTestSuite(t)
			appCli := getFakeAppCli()
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
			err := r.Exec(ctx, appCli, cmd)
			if tt.expErr != errAny {
				assert.Equal(tt.expErr, err)
			} else {
				assert.Error(err)
			}
		})
	}
}
