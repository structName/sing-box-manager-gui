package daemon

import (
	"errors"
	"syscall"
)

const exitCodeAddressInUse = 98

// ExitCodeForRunError maps server startup failures to process exit codes.
func ExitCodeForRunError(err error) int {
	if errors.Is(err, syscall.EADDRINUSE) {
		return exitCodeAddressInUse
	}
	return 1
}
