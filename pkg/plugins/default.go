// Package plugins provides launchr core plugins.
package plugins

import (
	// Default launchr plugins to include for launchr functionality.
	_ "github.com/launchrctl/launchr/pkg/plugins/builder"
	_ "github.com/launchrctl/launchr/pkg/plugins/yamldiscovery"
)
