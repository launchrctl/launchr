// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/launchrctl/launchr/core/driver (interfaces: ContainerRunner)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	"github.com/launchrctl/launchr/core/driver"
)

// MockContainerRunner is a mock of ContainerRunner interface.
type MockContainerRunner struct {
	ctrl     *gomock.Controller
	recorder *MockContainerRunnerMockRecorder
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
func (m *MockContainerRunner) ContainerAttach(arg0 context.Context, arg1 string, arg2 driver.ContainerAttachOptions) (*driver.ContainerInOut, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerAttach", arg0, arg1, arg2)
	ret0, _ := ret[0].(*driver.ContainerInOut)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContainerAttach indicates an expected call of ContainerAttach.
func (mr *MockContainerRunnerMockRecorder) ContainerAttach(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerAttach", reflect.TypeOf((*MockContainerRunner)(nil).ContainerAttach), arg0, arg1, arg2)
}

// ContainerCreate mocks base method.
func (m *MockContainerRunner) ContainerCreate(arg0 context.Context, arg1 driver.ContainerCreateOptions) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerCreate", arg0, arg1)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContainerCreate indicates an expected call of ContainerCreate.
func (mr *MockContainerRunnerMockRecorder) ContainerCreate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerCreate", reflect.TypeOf((*MockContainerRunner)(nil).ContainerCreate), arg0, arg1)
}

// ContainerExecResize mocks base method.
func (m *MockContainerRunner) ContainerExecResize(arg0 context.Context, arg1 string, arg2 driver.ResizeOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerExecResize", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerExecResize indicates an expected call of ContainerExecResize.
func (mr *MockContainerRunnerMockRecorder) ContainerExecResize(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerExecResize", reflect.TypeOf((*MockContainerRunner)(nil).ContainerExecResize), arg0, arg1, arg2)
}

// ContainerKill mocks base method.
func (m *MockContainerRunner) ContainerKill(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerKill", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerKill indicates an expected call of ContainerKill.
func (mr *MockContainerRunnerMockRecorder) ContainerKill(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerKill", reflect.TypeOf((*MockContainerRunner)(nil).ContainerKill), arg0, arg1, arg2)
}

// ContainerList mocks base method.
func (m *MockContainerRunner) ContainerList(arg0 context.Context, arg1 driver.ContainerListOptions) []driver.ContainerListResult {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerList", arg0, arg1)
	ret0, _ := ret[0].([]driver.ContainerListResult)
	return ret0
}

// ContainerList indicates an expected call of ContainerList.
func (mr *MockContainerRunnerMockRecorder) ContainerList(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerList", reflect.TypeOf((*MockContainerRunner)(nil).ContainerList), arg0, arg1)
}

// ContainerRemove mocks base method.
func (m *MockContainerRunner) ContainerRemove(arg0 context.Context, arg1 string, arg2 driver.ContainerRemoveOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerRemove", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerRemove indicates an expected call of ContainerRemove.
func (mr *MockContainerRunnerMockRecorder) ContainerRemove(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerRemove", reflect.TypeOf((*MockContainerRunner)(nil).ContainerRemove), arg0, arg1, arg2)
}

// ContainerResize mocks base method.
func (m *MockContainerRunner) ContainerResize(arg0 context.Context, arg1 string, arg2 driver.ResizeOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerResize", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerResize indicates an expected call of ContainerResize.
func (mr *MockContainerRunnerMockRecorder) ContainerResize(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerResize", reflect.TypeOf((*MockContainerRunner)(nil).ContainerResize), arg0, arg1, arg2)
}

// ContainerStart mocks base method.
func (m *MockContainerRunner) ContainerStart(arg0 context.Context, arg1 string, arg2 driver.ContainerStartOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStart", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerStart indicates an expected call of ContainerStart.
func (mr *MockContainerRunnerMockRecorder) ContainerStart(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStart", reflect.TypeOf((*MockContainerRunner)(nil).ContainerStart), arg0, arg1, arg2)
}

// ContainerStop mocks base method.
func (m *MockContainerRunner) ContainerStop(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStop", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerStop indicates an expected call of ContainerStop.
func (mr *MockContainerRunnerMockRecorder) ContainerStop(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStop", reflect.TypeOf((*MockContainerRunner)(nil).ContainerStop), arg0, arg1)
}

// ContainerWait mocks base method.
func (m *MockContainerRunner) ContainerWait(arg0 context.Context, arg1 string, arg2 driver.ContainerWaitOptions) (<-chan driver.ContainerWaitResponse, <-chan error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerWait", arg0, arg1, arg2)
	ret0, _ := ret[0].(<-chan driver.ContainerWaitResponse)
	ret1, _ := ret[1].(<-chan error)
	return ret0, ret1
}

// ContainerWait indicates an expected call of ContainerWait.
func (mr *MockContainerRunnerMockRecorder) ContainerWait(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerWait", reflect.TypeOf((*MockContainerRunner)(nil).ContainerWait), arg0, arg1, arg2)
}

// ImageEnsure mocks base method.
func (m *MockContainerRunner) ImageEnsure(arg0 context.Context, arg1 driver.ImageOptions) (*driver.ImageStatusResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ImageEnsure", arg0, arg1)
	ret0, _ := ret[0].(*driver.ImageStatusResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ImageEnsure indicates an expected call of ImageEnsure.
func (mr *MockContainerRunnerMockRecorder) ImageEnsure(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ImageEnsure", reflect.TypeOf((*MockContainerRunner)(nil).ImageEnsure), arg0, arg1)
}
