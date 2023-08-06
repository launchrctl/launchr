package launchr

// ServiceInfo provides service info for its initialization.
type ServiceInfo struct {
	ID string
}

// Service is a common interface for a service to register.
type Service interface {
	ServiceInfo() ServiceInfo
}
