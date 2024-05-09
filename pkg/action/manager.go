package action

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
)

// Manager handles actions and its execution.
type Manager interface {
	launchr.Service
	// All returns all actions copied and decorated.
	All() map[string]*Action
	// AllRef returns all original action values from the storage.
	// Use it only if you need to read-only actions without allocations. It may be unsafe to read/write the map.
	// If you need to run actions, use Get or All, it will provide configured for run Action.
	AllRef() map[string]*Action
	// Get returns a copy of an action from the manager with default decorators.
	Get(id string) (*Action, bool)
	// GetRef returns an original action value from the storage.
	GetRef(id string) (*Action, bool)
	// AddValueProcessor adds processor to list of available processors
	AddValueProcessor(name string, vp ValueProcessor)
	// GetValueProcessors returns list of available processors
	GetValueProcessors() map[string]ValueProcessor
	// Decorate decorates an action with given behaviors and returns its copy.
	// If functions withFn are not provided, default functions are applied.
	Decorate(a *Action, withFn ...DecorateWithFn) *Action
	// Add saves an action in the manager.
	Add(*Action)
	// DefaultRunEnvironment provides the default action run environment.
	DefaultRunEnvironment(a *Action) RunEnvironment

	// Run executes an action in foreground.
	Run(ctx context.Context, a *Action) (RunInfo, error)
	// RunBackground executes an action in background.
	RunBackground(ctx context.Context, a *Action) (RunInfo, chan error)
	// RunInfoByAction returns all running actions by action id.
	RunInfoByAction(aid string) []RunInfo
	// RunInfoByID returns an action matching run id.
	RunInfoByID(id string) (RunInfo, bool)
}

// DecorateWithFn is a type alias for functions accepted in a Manager.Decorate interface method.
type DecorateWithFn = func(m Manager, a *Action)

type actionManagerMap struct {
	actionStore map[string]*Action
	runStore    map[string]RunInfo // @todo consider persistent storage
	mx          sync.Mutex
	mxRun       sync.Mutex
	dwFns       []DecorateWithFn
	processors  map[string]ValueProcessor
}

// NewManager constructs a new action manager.
func NewManager(withFns ...DecorateWithFn) Manager {
	return &actionManagerMap{
		actionStore: make(map[string]*Action),
		runStore:    make(map[string]RunInfo),
		dwFns:       withFns,
		processors:  make(map[string]ValueProcessor),
	}
}

func (m *actionManagerMap) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{}
}

func (m *actionManagerMap) Add(a *Action) {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.actionStore[(*a).GetID()] = a
}

func (m *actionManagerMap) AllRef() map[string]*Action {
	m.mx.Lock()
	defer m.mx.Unlock()
	return copyMap(m.actionStore)
}

func (m *actionManagerMap) All() map[string]*Action {
	ret := m.AllRef()
	for k, v := range ret {
		ret[k] = m.Decorate(v, m.dwFns...)
	}
	return ret
}

func (m *actionManagerMap) Get(id string) (*Action, bool) {
	a, ok := m.GetRef(id)
	// Process action with default decorators and return a copy to have an isolated scope.
	return m.Decorate(a, m.dwFns...), ok
}

func (m *actionManagerMap) GetRef(id string) (*Action, bool) {
	m.mx.Lock()
	defer m.mx.Unlock()
	a, ok := m.actionStore[id]
	return a, ok
}

func (m *actionManagerMap) AddValueProcessor(name string, vp ValueProcessor) {
	if _, ok := m.processors[name]; ok {
		panic(fmt.Sprintf("processor `%q` with the same name already exists", name))
	}

	m.processors[name] = vp
}

func (m *actionManagerMap) GetValueProcessors() map[string]ValueProcessor {
	return m.processors
}

func (m *actionManagerMap) Decorate(a *Action, withFns ...DecorateWithFn) *Action {
	if a == nil {
		return nil
	}
	if withFns == nil {
		withFns = m.dwFns
	}
	act := (*a).Clone()
	for _, fn := range withFns {
		fn(m, &act)
	}
	return &act
}

func (m *actionManagerMap) DefaultRunEnvironment(a *Action) RunEnvironment {
	var env RunEnvironment

	switch (*a).(type) {
	case *CallbackAction:
		env = NewFunctionEnvironment()
	case *ContainerAction:
		env = NewDockerEnvironment()
	}

	return env
}

// RunInfo stores information about a running action.
type RunInfo struct {
	ID     string
	Action *Action
	Status string
	// @todo add more info for status like error message or exit code. Or have it in output.
}

func (m *actionManagerMap) registerRun(a *Action) RunInfo {
	// @todo rethink the implementation
	m.mxRun.Lock()
	defer m.mxRun.Unlock()
	// @todo validate the action is actually running and the method was not just incorrectly requested
	id := strconv.FormatInt(time.Now().Unix(), 10) + "-" + (*a).GetID()
	ri := RunInfo{
		ID:     id,
		Action: a,
		Status: "created",
	}
	m.runStore[id] = ri
	return ri
}

func (m *actionManagerMap) updateRunStatus(id string, st string) {
	m.mxRun.Lock()
	defer m.mxRun.Unlock()
	if ri, ok := m.runStore[id]; ok {
		ri.Status = st
		m.runStore[id] = ri
	}
}

func (m *actionManagerMap) Run(ctx context.Context, a *Action) (RunInfo, error) {
	// @todo add the same status change info
	return m.registerRun(a), (*a).Execute(ctx)
}

func (m *actionManagerMap) RunBackground(ctx context.Context, a *Action) (RunInfo, chan error) {
	ri := m.registerRun(a)
	chErr := make(chan error)
	go func() {
		m.updateRunStatus(ri.ID, "running")
		err := (*a).Execute(ctx)
		chErr <- err
		close(chErr)
		if err != nil {
			m.updateRunStatus(ri.ID, "error")
		} else {
			m.updateRunStatus(ri.ID, "finished")
		}
	}()
	// @todo rethink returned values.
	return ri, chErr
}

func (m *actionManagerMap) RunInfoByAction(aid string) []RunInfo {
	m.mxRun.Lock()
	defer m.mxRun.Unlock()
	run := make([]RunInfo, 0, len(m.runStore)/2)
	for _, v := range m.runStore {
		if (*v.Action).GetID() == aid {
			run = append(run, v)
		}
	}
	return run
}

func (m *actionManagerMap) RunInfoByID(id string) (RunInfo, bool) {
	m.mxRun.Lock()
	defer m.mxRun.Unlock()
	ri, ok := m.runStore[id]
	return ri, ok
}

// WithDefaultRunEnvironment adds a default RunEnvironment for an action.
func WithDefaultRunEnvironment(m Manager, a *Action) {
	(*a).SetRunEnvironment(m.DefaultRunEnvironment(a))
}

// WithContainerRunEnvironmentConfig configures a ContainerRunEnvironment.
func WithContainerRunEnvironmentConfig(cfg launchr.Config, prefix string) DecorateWithFn {
	r := LaunchrConfigImageBuildResolver{cfg}
	ccr := NewImageBuildCacheResolver(cfg)
	return func(m Manager, a *Action) {
		if env, ok := (*a).GetRunEnvironment().(ContainerRunEnvironment); ok {
			env.AddImageBuildResolver(r)
			env.SetImageBuildCacheResolver(ccr)
			env.SetContainerNameProvider(ContainerNameProvider{Prefix: prefix, RandomSuffix: true})
		}
	}
}

// WithValueProcessors sets processors for action from manager.
func WithValueProcessors() DecorateWithFn {
	return func(m Manager, a *Action) {
		(*a).SetProcessors(m.GetValueProcessors())
	}
}
