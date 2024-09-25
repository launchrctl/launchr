package cli

import (
	"github.com/launchrctl/launchr/internal/launchr"
)

// StandardStreams sets a cli in, out and err streams with the standard streams.
// Deprecated: use [launchr.StandardStreams]
func StandardStreams() launchr.Streams { return launchr.StandardStreams() }

// NoopStreams provides streams like /dev/null.
// Deprecated: use [launchr.NoopStreams]
func NoopStreams() launchr.Streams { return launchr.NoopStreams() }
