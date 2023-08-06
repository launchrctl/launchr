package action

import (
	"github.com/launchrctl/launchr/internal/launchr"
)

// Manager handles actions.
type Manager interface {
	launchr.Service
	Add(*Command)
	All() map[string]*Command
}

type actionManagerMap map[string]*Command

// NewManager constructs a new action manager.
func NewManager() Manager { return make(actionManagerMap) }

func (m actionManagerMap) ServiceInfo() launchr.ServiceInfo {
	return launchr.ServiceInfo{ID: "action_manager"}
}

func (m actionManagerMap) Add(cmd *Command) {
	m[cmd.CommandName] = cmd
}

func (m actionManagerMap) All() map[string]*Command { return m }
