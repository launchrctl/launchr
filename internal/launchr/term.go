package launchr

import (
	"io"
	"reflect"

	"github.com/pterm/pterm"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var defaultTerm *Terminal

// DefaultTextPrinter is a printer with a context of language.
// Currently only used in [jsonschema] package and not exported outside the repo.
// Looks promising in the future for translations.
var DefaultTextPrinter = message.NewPrinter(language.English)

func init() {
	// Initialize the default printer.
	defaultTerm = NewTerminal()
	// Do not output anything when not in the app, e.g. in tests.
	defaultTerm.DisableOutput()
}

// NewTerminal creates a new instance of [Terminal]
func NewTerminal() *Terminal {
	return &Terminal{
		p: []TextPrinter{
			printerBasic:   newPTermBasicPrinter(pterm.DefaultBasicText),
			printerInfo:    newPTermPrefixPrinter(pterm.Info),
			printerWarning: newPTermPrefixPrinter(pterm.Warning),
			printerSuccess: newPTermPrefixPrinter(pterm.Success),
			printerError:   newPTermPrefixPrinter(pterm.Error),
		},
		enabled: true,
	}
}

// Predefined keys of terminal printers.
const (
	printerBasic   int = iota // printerBasic prints without styles.
	printerInfo               // printerInfo prints with INFO prefix.
	printerWarning            // printerWarning prints with WARNING prefix.
	printerSuccess            // printerSuccess prints with SUCCESS prefix.
	printerError              // printerError prints with ERROR prefix.
)

// TextPrinter contains methods to print formatted text to the console or return it as a string.
type TextPrinter interface {
	// SetOutput sets where the output will be printed.
	SetOutput(w io.Writer)

	// Print formats using the default formats for its operands and writes to standard output.
	// Spaces are added between operands when neither is a string.
	Print(a ...any)

	// Println formats using the default formats for its operands and writes to standard output.
	// Spaces are always added between operands and a newline is appended.
	Println(a ...any)

	// Printf formats according to a format specifier and writes to standard output.
	Printf(format string, a ...any)

	// Printfln formats according to a format specifier and writes to standard output.
	// Spaces are always added between operands and a newline is appended.
	Printfln(format string, a ...any)
}

// Constructors to copy default pterm printers.
func newPTermBasicPrinter(p pterm.BasicTextPrinter) TextPrinter { return &ptermPrinter{&p} }
func newPTermPrefixPrinter(p pterm.PrefixPrinter) TextPrinter   { return &ptermPrinter{&p} }

type ptermPrinter struct {
	pterm pterm.TextPrinter
}

func (p *ptermPrinter) Print(a ...any)                   { p.pterm.Print(a...) }
func (p *ptermPrinter) Println(a ...any)                 { p.pterm.Println(a...) }
func (p *ptermPrinter) Printf(format string, a ...any)   { p.pterm.Printf(format, a...) }
func (p *ptermPrinter) Printfln(format string, a ...any) { p.pterm.Printfln(format, a...) }
func (p *ptermPrinter) SetOutput(w io.Writer) {
	// Call p.pterm.WithWriter(w)
	// All pterm structs have this method, but not in the interface.
	// To reduce repetitive code, we use reflect.
	v := reflect.ValueOf(p.pterm)
	method := v.MethodByName("WithWriter")
	if !method.IsValid() {
		panic("WithWriter is not implemented for this pterm.TextPrinter")
	}
	result := method.Call([]reflect.Value{reflect.ValueOf(w)})
	// Replace old printer by new one as WithWriter returns fresh copy of struct.
	p.pterm = result[0].Interface().(pterm.TextPrinter)
}

// Terminal prints formatted text to the console.
type Terminal struct {
	w io.Writer     // w is used to simplify usage of writers in underlying printers.
	p []TextPrinter // p contains styled printers.

	enabled bool // enabled disables output to the console if set to false.
}

// Term returns default [Terminal] to print application messages to the console.
func Term() *Terminal {
	return defaultTerm
}

// EnableOutput enables the output.
func (t *Terminal) EnableOutput() {
	pterm.EnableOutput()
	t.enabled = true
}

// DisableOutput disables the output.
func (t *Terminal) DisableOutput() {
	pterm.DisableOutput()
	t.enabled = false
}

// SetOutput sets an output to target writer.
func (t *Terminal) SetOutput(w io.Writer) {
	t.w = w
	// Ensure underlying printers use self.
	// Used to simplify update of writers in the printers.
	for i := 0; i < len(t.p); i++ {
		t.p[i].SetOutput(t)
	}
}

// Write implements [io.Writer] interface.
func (t *Terminal) Write(p []byte) (int, error) {
	if !t.enabled {
		return io.Discard.Write(p)
	}
	return t.w.Write(p)
}

// Print implements [TextPrinter] interface.
func (t *Terminal) Print(a ...any) {
	t.Basic().Print(a...)
}

// Println implements [TextPrinter] interface.
func (t *Terminal) Println(a ...any) {
	t.Basic().Println(a...)
}

// Printf implements [TextPrinter] interface.
func (t *Terminal) Printf(format string, a ...any) {
	t.Basic().Printf(format, a...)
}

// Printfln implements [TextPrinter] interface.
func (t *Terminal) Printfln(format string, a ...any) {
	t.Basic().Printfln(format, a...)
}

// Basic returns a default basic printer.
func (t *Terminal) Basic() TextPrinter {
	return t.p[printerBasic]
}

// Info returns a prefixed printer, which can be used to print text with an "info" prefix.
func (t *Terminal) Info() TextPrinter {
	return t.p[printerInfo]
}

// Warning returns a prefixed printer, which can be used to print text with a "warning" prefix.
func (t *Terminal) Warning() TextPrinter {
	return t.p[printerWarning]
}

// Success returns a prefixed printer, which can be used to print text with a "success" Prefix.
func (t *Terminal) Success() TextPrinter {
	return t.p[printerSuccess]
}

// Error returns a prefixed printer, which can be used to print text with an "error" prefix.
func (t *Terminal) Error() TextPrinter {
	return t.p[printerError]
}
