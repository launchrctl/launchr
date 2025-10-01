package launchr

import (
	"fmt"
	"reflect"
)

// ServiceInfo provides service info for its initialization.
type ServiceInfo struct {
	pkgPath  string
	typeName string
}

func (s ServiceInfo) String() string {
	return s.pkgPath + "." + s.typeName
}

// Service is a common interface for a service to register.
type Service interface {
	ServiceInfo() ServiceInfo
}

// ServiceCreate allows creating a service using a ServiceManager.
// TODO: Merge with Service when refactored.
type ServiceCreate interface {
	Service
	ServiceCreate(svc *ServiceManager) Service
}

// InitServiceInfo sets private fields for internal usage only.
func InitServiceInfo(si *ServiceInfo, s Service) {
	si.pkgPath, si.typeName = GetTypePkgPathName(s)
}

// ServiceManager is a basic Dependency Injection container storing registered [Service].
type ServiceManager struct {
	services map[ServiceInfo]Service
}

// NewServiceManager initializes ServiceManager.
func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		services: make(map[ServiceInfo]Service),
	}
}

// Add registers a service.
// Panics if a service is not unique.
func (sm *ServiceManager) Add(s Service) {
	info := sm.serviceInfo(s)
	if _, ok := sm.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info))
	}
	sm.services[info] = s
}

// Get retrieves a service of type [v] and assigns it to [v].
// Panics if a service is not found.
func (sm *ServiceManager) Get(v any) {
	// Check v is a pointer and implements [Service] to set a value later.
	t := reflect.TypeOf(v)
	isPtr := t != nil && t.Kind() == reflect.Pointer
	var stype reflect.Type
	if isPtr {
		stype = t.Elem()
	}

	// v must be [Service] but can't equal it because all elements implement it
	// and the first value will always be returned.
	intService := reflect.TypeOf((*Service)(nil)).Elem()
	if !isPtr || !stype.Implements(intService) || stype == intService {
		panic(fmt.Errorf("argument must be a pointer to a type (interface) implementing Service, %q given", t))
	}
	// Get service by service info.
	vsvc := reflect.ValueOf(v).Elem().Interface().(Service)
	srv, ok := sm.services[sm.serviceInfo(vsvc)]
	if ok && sm.tryToAssignService(srv, v) {
		return
	}

	// Find the service by type in the registered.
	for _, srv = range sm.services {
		if sm.tryToAssignService(srv, v) {
			return
		}
	}

	// Try to create and register a service if possible.
	if c, ok := vsvc.(ServiceCreate); ok {
		newSvc := c.ServiceCreate(sm)
		if sm.tryToAssignService(newSvc, v) {
			sm.Add(newSvc)
			return
		}
	}
	panic(fmt.Sprintf("service %q does not exist", stype))
}

func (sm *ServiceManager) serviceInfo(s Service) ServiceInfo {
	info := s.ServiceInfo()
	InitServiceInfo(&info, s)
	return info
}

func (sm *ServiceManager) tryToAssignService(s Service, v any) bool {
	stype := reflect.TypeOf(v).Elem()
	st := reflect.TypeOf(s)
	if st.AssignableTo(stype) {
		reflect.ValueOf(v).Elem().Set(reflect.ValueOf(s))
		return true
	}
	return false
}
