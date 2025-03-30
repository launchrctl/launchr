package action

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/launchrctl/launchr/internal/launchr"
	"github.com/launchrctl/launchr/pkg/driver"
)

// DiscoverActionsFn defines a function to discover actions.
type DiscoverActionsFn func(ctx context.Context) ([]*Action, error)

// Manager handles actions and its execution.
type Manager interface {
	launchr.Service
	// All returns all actions copied and decorated.
	All() map[string]*Action
	// Get returns a copy of an action from the manager with default decorators.
	Get(id string) (*Action, bool)
	// Add saves an action in the manager.
	Add(*Action) error
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
	// This id provider will be used as default on [Action] discovery process.
	SetActionIDProvider(p IDProvider)

	// AddValueProcessor adds processor to list of available processors
	AddValueProcessor(name string, vp ValueProcessor)
	// GetValueProcessors returns list of available processors
	GetValueProcessors() map[string]ValueProcessor

	// AddDiscovery registers a discovery callback to find actions.
	AddDiscovery(DiscoverActionsFn)
	// SetDiscoveryTimeout sets discovery timeout to stop on long-running callbacks.
	SetDiscoveryTimeout(timeout time.Duration)

	RunManager
}

// RunManager runs actions and stores runtime information about them.
type RunManager interface {
	// Run executes an action in foreground.
	Run(ctx context.Context, a *Action) (RunInfo, error)
	// RunBackground executes an action in background.
	RunBackground(ctx context.Context, a *Action, runID string) (RunInfo, chan error)
	// RunInfoByAction returns all running actions by action id.
	RunInfoByAction(aid string) []RunInfo
	// RunInfoByID returns an action matching run id.
	RunInfoByID(id string) (RunInfo, bool)
}

// ManagerUnsafe is an extension of the [Manager] interface that provides unsafe access to actions.
// Warning: Use this with caution!
type ManagerUnsafe interface {
	Manager
	// AllUnsafe returns all original action values from the storage.
	// Use this method only if you need read-only access to the actions without allocating new memory.
	// Warning: It is unsafe to manipulate these actions directly as they are the original instances
	// affecting the entire application.
	// Normally, for action execution you should use the [Manager.Get] or [Manager.All] methods,
	// which provide actions configured for execution.
	AllUnsafe() map[string]*Action
	// GetUnsafe returns the original action value from the storage.
	GetUnsafe(id string) (*Action, bool)
}

// DecorateWithFn is a type alias for functions accepted in a [Manager.Decorate] interface method.
type DecorateWithFn = func(m Manager, a *Action)

type actionManagerMap struct {
	actionStore   map[string]*Action
	actionAliases map[string]string
	mx            sync.Mutex
	dwFns         []DecorateWithFn
	processors    map[string]ValueProcessor
	idProvider    IDProvider

	// Actions discovery.
	discoveryFns []DiscoverActionsFn
	discoverySeq *launchr.SliceSeqStateful[DiscoverActionsFn]
	discTimeout  time.Duration

	runManagerMap
}

// NewManager constructs a new action manager.
func NewManager(withFns ...DecorateWithFn) Manager {
	return &actionManagerMap{
		actionStore:   make(map[string]*Action),
		actionAliases: make(map[string]string),
		dwFns:         withFns,
		processors:    make(map[string]ValueProcessor),

		discTimeout: 10 * time.Second,

		runManagerMap: runManagerMap{
			runStore: make(map[string]RunInfo),
		},
	}
}

func (m *actionManagerMap) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{}
}

func (m *actionManagerMap) Add(a *Action) error {
	m.mx.Lock()
	defer m.mx.Unlock()
	return m.add(a)
}

func (m *actionManagerMap) add(a *Action) error {
	// Check action loads properly.
	def, err := a.Raw()
	if err != nil {
		return err
	}
	// Collect action aliases.
	for _, alias := range def.Action.Aliases {
		id, ok := m.actionAliases[alias]
		if ok {
			return fmt.Errorf("alias %q is already defined by %q", alias, id)
		}
		m.actionAliases[alias] = a.ID
	}
	// Set action related processors.
	err = a.SetProcessors(m.GetValueProcessors())
	if err != nil {
		// Skip action because the definition is not correct.
		return err
	}
	if dup, ok := m.actionStore[a.ID]; ok {
		launchr.Log().Debug("action was overridden by another declaration",
			"action_id", a.ID,
			"old", dup.Filepath(),
			"new", a.Filepath(),
		)
	}
	m.actionStore[a.ID] = a
	return nil
}

func (m *actionManagerMap) AllUnsafe() map[string]*Action {
	m.mx.Lock()
	defer m.mx.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), m.discTimeout)
	defer cancel()
	_ = m.finalizeDiscovery(ctx)
	return maps.Clone(m.actionStore)
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

func (m *actionManagerMap) GetUnsafe(id string) (a *Action, ok bool) {
	m.mx.Lock()
	defer m.mx.Unlock()

	a, ok = m.get(id)
	if ok {
		return a, ok
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.discTimeout)
	defer cancel()
	for fn := range m.discoverySeq.Seq() {
		if err := m.callDiscoveryFn(ctx, fn); err != nil {
			continue
		}

		a, ok = m.get(id)
		if ok {
			return a, ok
		}
	}

	return a, ok
}

func (m *actionManagerMap) get(id string) (*Action, bool) {
	id = m.GetIDFromAlias(id)
	a, ok := m.actionStore[id]
	return a, ok
}

func (m *actionManagerMap) SetDiscoveryTimeout(timeout time.Duration) {
	m.discTimeout = timeout
}

func (m *actionManagerMap) AddDiscovery(fn DiscoverActionsFn) {
	if m.discoveryFns == nil {
		m.discoveryFns = make([]DiscoverActionsFn, 0, 1)
		m.discoverySeq = launchr.NewSliceSeqStateful(&m.discoveryFns)
	}
	m.discoveryFns = append(m.discoveryFns, fn)
}

func (m *actionManagerMap) finalizeDiscovery(ctx context.Context) error {
	errs := make([]error, 0)
	for fn := range m.discoverySeq.Seq() {
		err := m.callDiscoveryFn(ctx, fn)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *actionManagerMap) callDiscoveryFn(ctx context.Context, fn DiscoverActionsFn) error {
	actions, err := fn(ctx)
	if err != nil {
		return err
	}
	// Add discovered actions.
	for _, a := range actions {
		err = m.add(a)
		if err != nil {
			launchr.Log().Warn("action was skipped due to error", "action_id", a.ID, "error", err)
			launchr.Term().Warning().Printfln("Action %q was skipped:\n%v", a.ID, err)
			continue
		}
	}
	return nil
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

// RunInfo stores information about a running action.
type RunInfo struct {
	ID     string
	Action *Action
	Status string
	// @todo add more info for status like error message or exit code. Or have it in output.
}

type runManagerMap struct {
	runStore map[string]RunInfo // @todo consider persistent storage
	mx       sync.Mutex
}

func (m *runManagerMap) registerRun(a *Action, id string) RunInfo {
	// @todo rethink the implementation
	m.mx.Lock()
	defer m.mx.Unlock()
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

func (m *runManagerMap) updateRunStatus(id string, st string) {
	m.mx.Lock()
	defer m.mx.Unlock()
	if ri, ok := m.runStore[id]; ok {
		ri.Status = st
		m.runStore[id] = ri
	}
}

func (m *runManagerMap) Run(ctx context.Context, a *Action) (RunInfo, error) {
	// @todo add the same status change info
	return m.registerRun(a, ""), a.Execute(ctx)
}

func (m *runManagerMap) RunBackground(ctx context.Context, a *Action, runID string) (RunInfo, chan error) {
	// @todo change runID to runOptions with possibility to create filestream names in webUI.
	ri := m.registerRun(a, runID)
	chErr := make(chan error)
	go func() {
		m.updateRunStatus(ri.ID, "running")
		err := a.Execute(ctx)
		chErr <- err
		close(chErr)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				m.updateRunStatus(ri.ID, "canceled")
			} else {
				m.updateRunStatus(ri.ID, "error")
			}
		} else {
			m.updateRunStatus(ri.ID, "finished")
		}
	}()
	// @todo rethink returned values.
	return ri, chErr
}

func (m *runManagerMap) RunInfoByAction(aid string) []RunInfo {
	m.mx.Lock()
	defer m.mx.Unlock()
	run := make([]RunInfo, 0, len(m.runStore)/2)
	for _, v := range m.runStore {
		if v.Action.ID == aid {
			run = append(run, v)
		}
	}
	return run
}

func (m *runManagerMap) RunInfoByID(id string) (RunInfo, bool) {
	m.mx.Lock()
	defer m.mx.Unlock()
	ri, ok := m.runStore[id]
	return ri, ok
}

// WithDefaultRuntime adds a default [Runtime] for an action.
func WithDefaultRuntime(cfg launchr.Config) DecorateWithFn {
	type configContainer struct {
		Runtime string `yaml:"runtime"`
	}
	var rtConfig configContainer
	err := cfg.Get("container", &rtConfig)
	if err != nil {
		launchr.Term().Warning().Printfln("configuration file field %q is malformed", "container")
	}
	return func(_ Manager, a *Action) {
		if a.Runtime() != nil {
			return
		}
		def, _ := a.Raw()
		switch def.Runtime.Type {
		case runtimeTypeContainer:
			var rt ContainerRuntime
			switch driver.Type(rtConfig.Runtime) {
			case driver.Kubernetes:
				rt = NewContainerRuntimeKubernetes()
			case driver.Docker:
				fallthrough
			default:
				rt = NewContainerRuntimeDocker()
			}
			a.SetRuntime(rt)
		case runtimeTypeShell:
			a.SetRuntime(NewShellRuntime())
		}
	}
}

// WithContainerRuntimeConfig configures a [ContainerRuntime].
func WithContainerRuntimeConfig(cfg launchr.Config, prefix string) DecorateWithFn {
	r := LaunchrConfigImageBuildResolver{cfg}
	ccr := NewImageBuildCacheResolver(cfg)
	return func(_ Manager, a *Action) {
		if env, ok := a.Runtime().(ContainerRuntime); ok {
			env.AddImageBuildResolver(r)
			env.SetImageBuildCacheResolver(ccr)
			env.SetContainerNameProvider(ContainerNameProvider{Prefix: prefix, RandomSuffix: true})
		}
	}
}
