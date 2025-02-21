package launchr

import "errors"

type cleanupFn func() error

var registeredCleanups []cleanupFn

// RegisterCleanupFn saves a function to be executed on Cleanup.
func RegisterCleanupFn(fn cleanupFn) {
	registeredCleanups = append(registeredCleanups, fn)
}

// Cleanup runs registered cleanup functions.
// It is run on the termination of the application.
// Consider it as a global defer.
func Cleanup() error {
	errs := make([]error, 0, len(registeredCleanups))
	// Run like defers.
	for i := len(registeredCleanups) - 1; i >= 0; i-- {
		errs = append(errs, registeredCleanups[i]())
	}
	return errors.Join(errs...)
}
