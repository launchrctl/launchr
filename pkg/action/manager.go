package action

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/log"
)

// Manager handles actions and its execution.
type Manager interface {
	launchr.Service
	// All returns all actions copied and decorated.
	All() map[string]*Action
	// AllRef returns all original action values from the storage.
	// Deprecated: use ManagerUnsafe.AllUnsafe instead.
	AllRef() map[string]*Action
	// Get returns a copy of an action from the manager with default decorators.
	Get(id string) (*Action, bool)
	// GetRef returns an original action value from the storage.
	// Deprecated: use ManagerUnsafe.GetUnsafe instead.
	GetRef(id string) (*Action, bool)
	// Add saves an action in the manager.
	Add(*Action)
	// Delete deletes the action from the manager.
	Delete(id string)
	// Decorate decorates an action with given behaviors and returns its copy.
	// If functions withFn are not provided, default functions are applied.
	Decorate(a *Action, withFn ...DecorateWithFn) *Action
	// GetIDFromAlias returns a real action ID by its alias. If not, returns alias.
	GetIDFromAlias(alias string) string

	// GetActionIDProvider returns global application action id provider.
	GetActionIDProvider() IDProvider
	// SetActionIDProvider sets global application action id provider.
	// This id provider will be used as default on Action discovery process.
	SetActionIDProvider(p IDProvider)

	// AddValueProcessor adds processor to list of available processors
	AddValueProcessor(name string, vp ValueProcessor)
	// GetValueProcessors returns list of available processors
	GetValueProcessors() map[string]ValueProcessor

	// DefaultRunEnvironment provides the default action run environment.
	DefaultRunEnvironment() RunEnvironment
	// Run executes an action in foreground.
	Run(ctx context.Context, a *Action) (RunInfo, error)
	// RunBackground executes an action in background.
	RunBackground(ctx context.Context, a *Action, runID string) (RunInfo, chan error)
	// RunInfoByAction returns all running actions by action id.
	RunInfoByAction(aid string) []RunInfo
	// RunInfoByID returns an action matching run id.
	RunInfoByID(id string) (RunInfo, bool)
}

// ManagerUnsafe is an extension of the Manager interface that provides unsafe access to actions.
// Warning: Use this with caution!
type ManagerUnsafe interface {
	Manager
	// AllUnsafe returns all original action values from the storage.
	// Use this method only if you need read-only access to the actions without allocating new memory.
	// Warning: It is unsafe to manipulate these actions directly as they are the original instances
	// affecting the entire application.
	// Normally, for action execution you should use the Manager.Get or Manager.All methods,
	// which provide actions configured for execution.
	AllUnsafe() map[string]*Action
	// GetUnsafe returns the original action value from the storage.
	GetUnsafe(id string) (*Action, bool)
}

// DecorateWithFn is a type alias for functions accepted in a Manager.Decorate interface method.
type DecorateWithFn = func(m Manager, a *Action)

type actionManagerMap struct {
	actionStore   map[string]*Action
	actionAliases map[string]string
	runStore      map[string]RunInfo // @todo consider persistent storage
	mx            sync.Mutex
	mxRun         sync.Mutex
	dwFns         []DecorateWithFn
	processors    map[string]ValueProcessor
	idProvider    IDProvider
}

// NewManager constructs a new action manager.
func NewManager(withFns ...DecorateWithFn) Manager {
	return &actionManagerMap{
		actionStore:   make(map[string]*Action),
		actionAliases: make(map[string]string),
		runStore:      make(map[string]RunInfo),
		dwFns:         withFns,
		processors:    make(map[string]ValueProcessor),
	}
}

func (m *actionManagerMap) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{}
}

func (m *actionManagerMap) Add(a *Action) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.actionStore[a.ID] = a

	// Collect action aliases.
	def, err := a.Raw()
	if err != nil {
		return
	}
	for _, alias := range def.Action.Aliases {
		id, ok := m.actionAliases[alias]
		if ok {
			log.Warn("Alias %q is already defined by %q", alias, id)
		} else {
			m.actionAliases[alias] = a.ID
		}
	}
}

func (m *actionManagerMap) AllUnsafe() map[string]*Action {
	m.mx.Lock()
	defer m.mx.Unlock()
	return copyMap(m.actionStore)
}

// Deprecated: use AllUnsafe instead.
func (m *actionManagerMap) AllRef() map[string]*Action {
	return m.AllUnsafe()
}

func (m *actionManagerMap) GetIDFromAlias(alias string) string {
	if id, ok := m.actionAliases[alias]; ok {
		return id
	}
	return alias
}

func (m *actionManagerMap) Delete(id string) {
	m.mx.Lock()
	defer m.mx.Unlock()
	_, ok := m.actionStore[id]
	if !ok {
		return
	}
	delete(m.actionStore, id)
	for _, idAlias := range m.actionAliases {
		if idAlias == id {
			delete(m.actionAliases, id)
		}
	}
}

func (m *actionManagerMap) All() map[string]*Action {
	ret := m.AllUnsafe()
	for k, v := range ret {
		ret[k] = m.Decorate(v, m.dwFns...)
	}
	return ret
}

func (m *actionManagerMap) Get(id string) (*Action, bool) {
	a, ok := m.GetUnsafe(id)
	// Process action with default decorators and return a copy to have an isolated scope.
	return m.Decorate(a, m.dwFns...), ok
}

func (m *actionManagerMap) GetUnsafe(id string) (*Action, bool) {
	m.mx.Lock()
	defer m.mx.Unlock()
	a, ok := m.actionStore[id]
	return a, ok
}

// Deprecated: use GetUnsafe instead.
func (m *actionManagerMap) GetRef(id string) (*Action, bool) {
	return m.GetUnsafe(id)
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
	a = a.Clone()
	for _, fn := range withFns {
		fn(m, a)
	}
	return a
}

func (m *actionManagerMap) GetActionIDProvider() IDProvider {
	if m.idProvider == nil {
		m.SetActionIDProvider(nil)
	}
	return m.idProvider
}

func (m *actionManagerMap) SetActionIDProvider(p IDProvider) {
	if p == nil {
		p = DefaultIDProvider{}
	}
	m.idProvider = p
}

func (m *actionManagerMap) DefaultRunEnvironment() RunEnvironment {
	return NewDockerEnvironment()
}

// RunInfo stores information about a running action.
type RunInfo struct {
	ID     string
	Action *Action
	Status string
	// @todo add more info for status like error message or exit code. Or have it in output.
}

func (m *actionManagerMap) registerRun(a *Action, id string) RunInfo {
	// @todo rethink the implementation
	m.mxRun.Lock()
	defer m.mxRun.Unlock()
	if id == "" {
		id = strconv.FormatInt(time.Now().Unix(), 10) + "-" + a.ID
	}
	// @todo validate the action is actually running and the method was not just incorrectly requested
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
	return m.registerRun(a, ""), a.Execute(ctx)
}

func (m *actionManagerMap) RunBackground(ctx context.Context, a *Action, runID string) (RunInfo, chan error) {
	// @todo change runID to runOptions with possibility to create filestream names in webUI.
	ri := m.registerRun(a, runID)
	chErr := make(chan error)
	go func() {
		m.updateRunStatus(ri.ID, "running")
		err := a.Execute(ctx)
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
		if v.Action.ID == aid {
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
	a.SetRunEnvironment(m.DefaultRunEnvironment())
}

// WithContainerRunEnvironmentConfig configures a ContainerRunEnvironment.
func WithContainerRunEnvironmentConfig(cfg launchr.Config, prefix string) DecorateWithFn {
	r := LaunchrConfigImageBuildResolver{cfg}
	ccr := NewImageBuildCacheResolver(cfg)
	return func(_ Manager, a *Action) {
		if env, ok := a.env.(ContainerRunEnvironment); ok {
			env.AddImageBuildResolver(r)
			env.SetImageBuildCacheResolver(ccr)
			env.SetContainerNameProvider(ContainerNameProvider{Prefix: prefix, RandomSuffix: true})
		}
	}
}

// WithValueProcessors sets processors for action from manager.
func WithValueProcessors() DecorateWithFn {
	return func(m Manager, a *Action) {
		a.SetProcessors(m.GetValueProcessors())
	}
}
