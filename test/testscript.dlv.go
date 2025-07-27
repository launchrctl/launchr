package test

import (
	"net"
	"path/filepath"
	"strconv"
	_ "unsafe" // Include an internal method of the testscript module.

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/launchrctl/launchr/internal/launchr"
)

// CmdDlv implements a custom testscript command for debugging with Delve
func CmdDlv(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("dlv command does not support negation")
	}

	if len(args) < 1 {
		ts.Fatalf("dlv: missing binary name\nUsage: dlv <binary> [args...]")
	}

	// Check if running in debug mode
	if !launchr.Version().Debug {
		ts.Fatalf("dlv command requires the tests to be run with debug flags")
	}

	command := args[0]
	binaryArgs := args[1:]
	if filepath.Base(command) == command {
		if lp, err := lookPath(command, ts.Getenv); err != nil {
			ts.Fatalf("error when looking for %s: %v", command, err)
		} else {
			command = lp
		}
	}

	// Find an available port
	port := findAvailablePort()

	// Log connection information
	ts.Logf("=== Delve Debug Server ===")
	ts.Logf("Debugging binary: %s", command)
	ts.Logf("Port: %d", port)
	ts.Logf("Connect with: dlv connect 127.0.0.1:%d", port)
	ts.Logf("GoLand Remote Debug: 127.0.0.1:%d", port)
	ts.Logf("=========================")

	// Build dlv command arguments
	cmdArgs := []string{
		"exec", command,
		"--listen=127.0.0.1:" + strconv.Itoa(port),
		"--headless=true",
		"--api-version=2",
		"--accept-multiclient",
	}

	// Add binary arguments if any
	if len(binaryArgs) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, binaryArgs...)
	}

	// Execute dlv using testscript's exec method
	_ = ts.Exec("dlv", cmdArgs...)
}

//go:linkname lookPath github.com/rogpeppe/go-internal/internal/os/execpath.Look
func lookPath(file string, getenv func(string) string) (string, error)

// findAvailablePort finds an available port starting from 2345
func findAvailablePort() int {
	for port := 2345; port <= 2355; port++ {
		if isPortAvailable(port) {
			return port
		}
	}
	return 2345 // fallback
}

// isPortAvailable checks if a port is available
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
