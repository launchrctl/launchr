//go:build windows

package launchr

import "os"

func isRuntimeSig(_ os.Signal) bool {
	return false
}
