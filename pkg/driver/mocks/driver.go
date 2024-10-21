// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/launchrctl/launchr/pkg/driver (interfaces: ContainerRunner)
//
// Generated by this command:
//
//	mockgen -destination=mocks/driver.go -package=mocks . ContainerRunner
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	io "io"
	reflect "reflect"

	container "github.com/docker/docker/api/types/container"
	image "github.com/docker/docker/api/types/image"
	driver "github.com/launchrctl/launchr/pkg/driver"
	types "github.com/launchrctl/launchr/pkg/types"
	gomock "go.uber.org/mock/gomock"
)

// MockContainerRunner is a mock of ContainerRunner interface.
type MockContainerRunner struct {
	ctrl     *gomock.Controller
	recorder *MockContainerRunnerMockRecorder
	isgomock struct{}
}

// MockContainerRunnerMockRecorder is the mock recorder for MockContainerRunner.
type MockContainerRunnerMockRecorder struct {
	mock *MockContainerRunner
}

// NewMockContainerRunner creates a new mock instance.
func NewMockContainerRunner(ctrl *gomock.Controller) *MockContainerRunner {
	mock := &MockContainerRunner{ctrl: ctrl}
	mock.recorder = &MockContainerRunnerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockContainerRunner) EXPECT() *MockContainerRunnerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockContainerRunner) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockContainerRunnerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockContainerRunner)(nil).Close))
}

// ContainerAttach mocks base method.
func (m *MockContainerRunner) ContainerAttach(ctx context.Context, cid string, opts container.AttachOptions) (*driver.ContainerInOut, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerAttach", ctx, cid, opts)
	ret0, _ := ret[0].(*driver.ContainerInOut)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContainerAttach indicates an expected call of ContainerAttach.
func (mr *MockContainerRunnerMockRecorder) ContainerAttach(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerAttach", reflect.TypeOf((*MockContainerRunner)(nil).ContainerAttach), ctx, cid, opts)
}

// ContainerCreate mocks base method.
func (m *MockContainerRunner) ContainerCreate(ctx context.Context, opts types.ContainerCreateOptions) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerCreate", ctx, opts)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContainerCreate indicates an expected call of ContainerCreate.
func (mr *MockContainerRunnerMockRecorder) ContainerCreate(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerCreate", reflect.TypeOf((*MockContainerRunner)(nil).ContainerCreate), ctx, opts)
}

// ContainerExecResize mocks base method.
func (m *MockContainerRunner) ContainerExecResize(ctx context.Context, cid string, opts container.ResizeOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerExecResize", ctx, cid, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerExecResize indicates an expected call of ContainerExecResize.
func (mr *MockContainerRunnerMockRecorder) ContainerExecResize(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerExecResize", reflect.TypeOf((*MockContainerRunner)(nil).ContainerExecResize), ctx, cid, opts)
}

// ContainerKill mocks base method.
func (m *MockContainerRunner) ContainerKill(ctx context.Context, cid, signal string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerKill", ctx, cid, signal)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerKill indicates an expected call of ContainerKill.
func (mr *MockContainerRunnerMockRecorder) ContainerKill(ctx, cid, signal any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerKill", reflect.TypeOf((*MockContainerRunner)(nil).ContainerKill), ctx, cid, signal)
}

// ContainerList mocks base method.
func (m *MockContainerRunner) ContainerList(ctx context.Context, opts types.ContainerListOptions) []types.ContainerListResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerList", ctx, opts)
	ret0, _ := ret[0].([]types.ContainerListResult)
	return ret0
}

// ContainerList indicates an expected call of ContainerList.
func (mr *MockContainerRunnerMockRecorder) ContainerList(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerList", reflect.TypeOf((*MockContainerRunner)(nil).ContainerList), ctx, opts)
}

// ContainerRemove mocks base method.
func (m *MockContainerRunner) ContainerRemove(ctx context.Context, cid string, opts container.RemoveOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerRemove", ctx, cid, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerRemove indicates an expected call of ContainerRemove.
func (mr *MockContainerRunnerMockRecorder) ContainerRemove(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerRemove", reflect.TypeOf((*MockContainerRunner)(nil).ContainerRemove), ctx, cid, opts)
}

// ContainerResize mocks base method.
func (m *MockContainerRunner) ContainerResize(ctx context.Context, cid string, opts container.ResizeOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerResize", ctx, cid, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerResize indicates an expected call of ContainerResize.
func (mr *MockContainerRunnerMockRecorder) ContainerResize(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerResize", reflect.TypeOf((*MockContainerRunner)(nil).ContainerResize), ctx, cid, opts)
}

// ContainerStart mocks base method.
func (m *MockContainerRunner) ContainerStart(ctx context.Context, cid string, opts types.ContainerStartOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStart", ctx, cid, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerStart indicates an expected call of ContainerStart.
func (mr *MockContainerRunnerMockRecorder) ContainerStart(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStart", reflect.TypeOf((*MockContainerRunner)(nil).ContainerStart), ctx, cid, opts)
}

// ContainerStatPath mocks base method.
func (m *MockContainerRunner) ContainerStatPath(ctx context.Context, cid, path string) (container.PathStat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStatPath", ctx, cid, path)
	ret0, _ := ret[0].(container.PathStat)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContainerStatPath indicates an expected call of ContainerStatPath.
func (mr *MockContainerRunnerMockRecorder) ContainerStatPath(ctx, cid, path any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStatPath", reflect.TypeOf((*MockContainerRunner)(nil).ContainerStatPath), ctx, cid, path)
}

// ContainerStop mocks base method.
func (m *MockContainerRunner) ContainerStop(ctx context.Context, cid string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStop", ctx, cid)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerStop indicates an expected call of ContainerStop.
func (mr *MockContainerRunnerMockRecorder) ContainerStop(ctx, cid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStop", reflect.TypeOf((*MockContainerRunner)(nil).ContainerStop), ctx, cid)
}

// ContainerWait mocks base method.
func (m *MockContainerRunner) ContainerWait(ctx context.Context, cid string, opts types.ContainerWaitOptions) (<-chan types.ContainerWaitResponse, <-chan error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerWait", ctx, cid, opts)
	ret0, _ := ret[0].(<-chan types.ContainerWaitResponse)
	ret1, _ := ret[1].(<-chan error)
	return ret0, ret1
}

// ContainerWait indicates an expected call of ContainerWait.
func (mr *MockContainerRunnerMockRecorder) ContainerWait(ctx, cid, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerWait", reflect.TypeOf((*MockContainerRunner)(nil).ContainerWait), ctx, cid, opts)
}

// CopyFromContainer mocks base method.
func (m *MockContainerRunner) CopyFromContainer(ctx context.Context, cid, srcPath string) (io.ReadCloser, container.PathStat, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyFromContainer", ctx, cid, srcPath)
	ret0, _ := ret[0].(io.ReadCloser)
	ret1, _ := ret[1].(container.PathStat)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CopyFromContainer indicates an expected call of CopyFromContainer.
func (mr *MockContainerRunnerMockRecorder) CopyFromContainer(ctx, cid, srcPath any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyFromContainer", reflect.TypeOf((*MockContainerRunner)(nil).CopyFromContainer), ctx, cid, srcPath)
}

// CopyToContainer mocks base method.
func (m *MockContainerRunner) CopyToContainer(ctx context.Context, cid, path string, content io.Reader, opts container.CopyToContainerOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyToContainer", ctx, cid, path, content, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// CopyToContainer indicates an expected call of CopyToContainer.
func (mr *MockContainerRunnerMockRecorder) CopyToContainer(ctx, cid, path, content, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyToContainer", reflect.TypeOf((*MockContainerRunner)(nil).CopyToContainer), ctx, cid, path, content, opts)
}

// ImageEnsure mocks base method.
func (m *MockContainerRunner) ImageEnsure(ctx context.Context, opts types.ImageOptions) (*types.ImageStatusResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ImageEnsure", ctx, opts)
	ret0, _ := ret[0].(*types.ImageStatusResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ImageEnsure indicates an expected call of ImageEnsure.
func (mr *MockContainerRunnerMockRecorder) ImageEnsure(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ImageEnsure", reflect.TypeOf((*MockContainerRunner)(nil).ImageEnsure), ctx, opts)
}

// ImageRemove mocks base method.
func (m *MockContainerRunner) ImageRemove(ctx context.Context, image string, opts image.RemoveOptions) (*types.ImageRemoveResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ImageRemove", ctx, image, opts)
	ret0, _ := ret[0].(*types.ImageRemoveResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ImageRemove indicates an expected call of ImageRemove.
func (mr *MockContainerRunnerMockRecorder) ImageRemove(ctx, image, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ImageRemove", reflect.TypeOf((*MockContainerRunner)(nil).ImageRemove), ctx, image, opts)
}

// Info mocks base method.
func (m *MockContainerRunner) Info(ctx context.Context) (types.SystemInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info", ctx)
	ret0, _ := ret[0].(types.SystemInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Info indicates an expected call of Info.
func (mr *MockContainerRunnerMockRecorder) Info(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockContainerRunner)(nil).Info), ctx)
}
