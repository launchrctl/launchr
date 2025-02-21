// Package launchr provides common app functionality.
package launchr

import (
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
)

// IsGen is an internal flag that indicates we are in Generate.
var IsGen = false

// GetTypePkgPathName returns type package path and name for internal usage.
func GetTypePkgPathName(v any) (string, string) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath(), t.Name()
}

// GetPluginByType returns specific plugins from the app.
func GetPluginByType[T Plugin](mngr PluginManager) []MapItem[PluginInfo, T] {
	// Collect plugins according to their weights.
	m := make(map[int][]MapItem[PluginInfo, T])
	cnt := 0
	for pi, p := range mngr.All() {
		p, ok := p.(T)
		if ok {
			item := MapItem[PluginInfo, T]{K: pi, V: p}
			m[pi.Weight] = append(m[pi.Weight], item)
			cnt++
		}
	}
	// Sort weight keys.
	weights := make([]int, 0, len(m))
	for w := range m {
		weights = append(weights, w)
	}
	sort.Ints(weights)
	// Merge all to a sorted list of plugins.
	// @todo maybe sort everything on init to optimize.
	res := make([]MapItem[PluginInfo, T], 0, cnt)
	for _, w := range weights {
		res = append(res, m[w]...)
	}
	return res
}

// IsCommandErrHelp checks if an error is a flag help err used for intercommunication.
func IsCommandErrHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

// EstimateTime returns a function that runs callback with
// the elapsed time between the call to timer and the call to
// the returned function. The returned function is intended to
// be used in a defer statement:
//
// defer EstimateTime("sum", func (diff time.Duration) { ... })().
func EstimateTime(fn func(diff time.Duration)) func() {
	start := time.Now()
	return func() {
		fn(time.Since(start))
	}
}

// IsSELinuxEnabled checks if selinux is enabled on the system.
func IsSELinuxEnabled() bool {
	// @todo it won't actually work with a remote environment.
	data, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}
	return string(data) == "1"
}

// CmdEarlyParsed is all parsed command information on early stage.
type CmdEarlyParsed struct {
	Command   string   // Command is the requested command.
	Args      []string // Args are all arguments provided in the command line.
	IsVersion bool     // IsVersion when version was requested.
	IsGen     bool     // IsGen when in generate mod.
}

// EarlyPeekCommand parses all available information during init stage.
func EarlyPeekCommand() CmdEarlyParsed {
	args := os.Args[1:]
	var isVersion bool
	var reqCmd string
	// Quick parse arguments to see if a version or help was requested.
	for i := 0; i < len(args); i++ {
		if args[i] == "--version" {
			isVersion = true
			break
		}
	}
	cmds := searchCommand(args)
	if len(cmds) > 0 {
		reqCmd = cmds[0]
	}

	return CmdEarlyParsed{
		Command:   reqCmd,
		Args:      args,
		IsVersion: isVersion,
		IsGen:     IsGen,
	}
}

func searchCommand(args []string) []string {
	if len(args) == 0 {
		return args
	}

	commands := []string{}

Loop:
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		switch {
		case s == "--":
			// "--" terminates the flags
			break Loop
		case strings.HasPrefix(s, "--") && !strings.Contains(s, "="):
			// If '--flag arg' then
			// delete arg from args.
			fallthrough // (do the same as below)
		case strings.HasPrefix(s, "-") && !strings.Contains(s, "=") && len(s) == 2:
			// If '-f arg' then
			// delete 'arg' from args or break the loop if len(args) <= 1.
			if len(args) <= 1 {
				break Loop
			}
			args = args[1:]
			continue
		case s != "" && !strings.HasPrefix(s, "-") && !strings.Contains(s, "="):
			commands = append(commands, s)
		}
	}

	return commands
}
