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

// InitServiceInfo sets private fields for internal usage only.
func InitServiceInfo(si *ServiceInfo, s Service) {
	si.pkgPath, si.typeName = GetTypePkgPathName(s)
}

// ServiceManager is a basic Dependency Injection container storing registered [Service].
type ServiceManager interface {
	// Add registers a service.
	// Panics if a service is not unique.
	Add(s Service)
	// Get retrieves a service of type [v] and assigns it to [v].
	// Panics if a service is not found.
	Get(v any)
}

type serviceManager struct {
	services map[ServiceInfo]Service
}

// NewServiceManager initializes ServiceManager.
func NewServiceManager() ServiceManager {
	return &serviceManager{
		services: make(map[ServiceInfo]Service),
	}
}

func (sm *serviceManager) Add(s Service) {
	info := s.ServiceInfo()
	InitServiceInfo(&info, s)
	if _, ok := sm.services[info]; ok {
		panic(fmt.Errorf("service %s already exists, review your code", info))
	}
	sm.services[info] = s
}

func (sm *serviceManager) Get(v any) {
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
	for _, srv := range sm.services {
		st := reflect.TypeOf(srv)
		if st.AssignableTo(stype) {
			reflect.ValueOf(v).Elem().Set(reflect.ValueOf(srv))
			return
		}
	}
	panic(fmt.Sprintf("service %q does not exist", stype))
}
