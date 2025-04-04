package actionscobra

import (
	"strings"
	"text/template"
	_ "unsafe" // Use unsafe to get template functions from cobra.

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/launchrctl/launchr/pkg/action"
)

//go:linkname templateFuncs github.com/spf13/cobra.templateFuncs
var templateFuncs template.FuncMap

type usageData struct {
	*cobra.Command
	Arguments      *pflag.FlagSet
	Options        *pflag.FlagSet
	RuntimeOptions *pflag.FlagSet
}

func getCmdUse(a *action.Action) string {
	def := a.ActionDef()
	parts := make([]string, len(def.Arguments)+1)
	parts[0] = a.ID
	for i, p := range def.Arguments {
		parts[i+1] = p.Name
		if !p.Required {
			parts[i+1] = "[" + p.Name + "]"
		}
	}
	return strings.Join(parts, " ")
}

func usageTplFn(a *action.Action) func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		def := a.ActionDef()

		var runtimeFlags action.ParametersList
		if env, ok := a.Runtime().(action.RuntimeFlags); ok {
			runtimeFlags = env.FlagsDefinition()
		}

		t := template.New("top")
		t.Funcs(templateFuncs)
		t.Funcs(template.FuncMap{
			"replaceActionArgsNameDashes": replaceActionParamNameDashes(def.Arguments),
		})
		template.Must(t.Parse(usageTemplate))
		return t.Execute(c.OutOrStderr(), usageData{
			Command:        c,
			Arguments:      getFlagsForParams(def.Arguments),
			Options:        getFlagsForParams(def.Options),
			RuntimeOptions: getFlagsForParams(runtimeFlags),
		})
	}
}

func replaceActionParamNameDashes(params action.ParametersList) func(s string) string {
	return func(s string) string {
		if len(params) == 0 {
			return s
		}
		oldnewArgs := make([]string, 0, len(params)*2)
		for _, param := range params {
			oldnewArgs = append(oldnewArgs, "--"+param.Name, param.Name)
		}
		return strings.NewReplacer(oldnewArgs...).Replace(s)
	}
}

func getFlagsForParams(params action.ParametersList) *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	for _, param := range params {
		_, _ = setFlag(flags, param)
	}
	return flags
}

const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{- if .Arguments}}

Arguments:
{{.Arguments.FlagUsages | replaceActionArgsNameDashes | trimTrailingWhitespaces}}{{end}}
{{- if .Options}}

Options:
{{.Options.FlagUsages | trimTrailingWhitespaces}}{{end}}
{{- if .RuntimeOptions}}

Action runtime options:
{{.RuntimeOptions.FlagUsages | trimTrailingWhitespaces}}{{end}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
