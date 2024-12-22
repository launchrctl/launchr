//go:build windows

package driver

import "os"

func isRuntimeSig(_ os.Signal) bool {
	return false
}
