//go:build windows

package action

import "os"

func isRuntimeSig(_ os.Signal) bool {
	return false
}
