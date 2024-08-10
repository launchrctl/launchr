// Package plugins provides launchr core plugins.
package plugins

import (
	// Default launchr plugins to include for launchr functionality.
	_ "github.com/launchrctl/launchr/plugins/actionnaming"
	_ "github.com/launchrctl/launchr/plugins/builder"
	_ "github.com/launchrctl/launchr/plugins/verbosity"
	_ "github.com/launchrctl/launchr/plugins/yamldiscovery"
)
