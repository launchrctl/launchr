package cli

import (
	"io"

	"github.com/launchrctl/launchr/internal/launchr"
)

// Streams is an interface which exposes the standard input and output streams
// Deprecated: use [launchr.Streams]
type Streams = launchr.Streams

// Out is an output stream used by the DockerCli to write normal program output.
// Deprecated: use [launchr.Out]
type Out = launchr.Out

// NewOut returns a new [Out] object from a [io.Writer]
// Deprecated: use [launchr.NewOut]
func NewOut(out io.Writer) *launchr.Out { return launchr.NewOut(out) }

// In is an input stream used by the DockerCli to read user input
// Deprecated: use [launchr.In]
type In = launchr.In

// NewIn returns a new [In] object from a [io.ReadCloser]
// Deprecated: use [launchr.NewIn]
func NewIn(in io.ReadCloser) *launchr.In { return launchr.NewIn(in) }
