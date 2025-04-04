package action

import (
	"path/filepath"
	"strings"
)

// IDProvider provides an ID for an action.
// It is used to generate an ID from an action declaration.
// [DefaultIDProvider] is the default implementation based on action filepath.
type IDProvider interface {
	GetID(a *Action) string
}

// DefaultIDProvider is a default action id provider.
// It generates action id by a filepath.
type DefaultIDProvider struct{}

// GetID implements [IDProvider] interface.
// It parses action filename and returns CLI command name.
// Empty string if the command name can't be generated.
func (idp DefaultIDProvider) GetID(a *Action) string {
	f := a.fpath
	s := filepath.Dir(f)
	// Support actions in root dir.
	if strings.HasPrefix(s, actionsDirname) {
		s = string(filepath.Separator) + s
	}
	i := strings.LastIndex(s, actionsSubdir)
	if i == -1 {
		return ""
	}
	s = s[:i] + strings.Replace(s[i:], actionsSubdir, ":", 1)
	s = strings.ReplaceAll(s, string(filepath.Separator), ".")
	s = strings.Trim(s, ".:")
	return s
}

// StringID is an [IDProvider] with constant string id.
type StringID string

// GetID implements [IDProvider] interface to return itself.
func (idp StringID) GetID(_ *Action) string {
	return string(idp)
}
