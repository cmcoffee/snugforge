# eflag
--
    import "github.com/cmcoffee/snugforge/eflag"

Package 'eflag' is a wrapper around Go's standard flag, it provides enhancments
for: Adding Header and Footer's to Usage. Adding Aliases to flags. (ie.. -d,
--debug) Enhances formatting for flag usage. Aside from that everything else is
standard from the flag library.

## Usage

```go
var (
	InlineArgs    = cmd.InlineArgs
	SyntaxName    = cmd.SyntaxName
	SetOutput     = cmd.SetOutput
	PrintDefaults = cmd.PrintDefaults
	Shorten       = cmd.Shorten
	String        = cmd.String
	StringVar     = cmd.StringVar
	Arg           = cmd.Arg
	Args          = cmd.Args
	Bool          = cmd.Bool
	BoolVar       = cmd.BoolVar
	Duration      = cmd.Duration
	DurationVar   = cmd.DurationVar
	Float64       = cmd.Float64
	Float64Var    = cmd.Float64Var
	Int           = cmd.Int
	IntVar        = cmd.IntVar
	Int64         = cmd.Int64
	Int64Var      = cmd.Int64Var
	Lookup        = cmd.Lookup
	Multi         = cmd.Multi
	MultiVar      = cmd.MultiVar
	NArg          = cmd.NArg
	NFlag         = cmd.NFlag
	Name          = cmd.Name
	Output        = cmd.Output
	Parsed        = cmd.Parsed
	Uint          = cmd.Uint
	UintVar       = cmd.UintVar
	Uint64        = cmd.Uint64
	Uint64Var     = cmd.Uint64Var
	Var           = cmd.Var
	Visit         = cmd.Visit
	VisitAll      = cmd.VisitAll
)
```

```go
var ErrHelp = flag.ErrHelp
```
ErrHelp is returned by Parse when the -help flag is encountered.

#### func  Footer

```go
func Footer(input string)
```
Footer sets the footer message for the command line interface. It updates the
'Footer' field of the global 'cmd' flag set.

#### func  Header

```go
func Header(input string)
```
Header sets the header string for the command line flags.

#### func  Parse

```go
func Parse() (err error)
```
Parse parses command-line arguments. It returns an error if parsing fails.

#### func  Usage

```go
func Usage()
```
Usage prints help information for the command.

#### type EFlagSet

```go
type EFlagSet struct {
	Header     string // Header presented at start of help.
	Footer     string // Footer presented at end of help.
	AdaptArgs  bool   // Reorders flags and arguments so flags come first, non-flag arguments second, unescapes arguments with '\' escape character.
	ShowSyntax bool   // Display Usage: line, InlineArgs will automatically display usage info.

	*flag.FlagSet
}
```

EFlagSet represents a flag set with extended features. It provides
functionalities for customizing flag parsing, handling arguments, and generating
help messages.

#### func  NewFlagSet

```go
func NewFlagSet(name string, errorHandling ErrorHandling) (output *EFlagSet)
```
NewFlagSet creates a new flag set with the given name and error handling policy.
It initializes the flag set and sets up the usage function.

#### func (*EFlagSet) Args

```go
func (s *EFlagSet) Args() []string
```
Args returns the non-flag arguments. It adapts the arguments by removing the
escape character '\' if AdaptArgs is true.

#### func (*EFlagSet) Bool

```go
func (E *EFlagSet) Bool(name string, usage string) *bool
```
Bool defines a boolean flag with the given name and usage string. It returns a
pointer to the boolean value.

#### func (*EFlagSet) BoolVar

```go
func (E *EFlagSet) BoolVar(p *bool, name string, usage string)
```
BoolVar defines a boolean flag with the given name and usage string. It sets the
default value of the flag to false if it is not set.

#### func (*EFlagSet) InlineArgs

```go
func (E *EFlagSet) InlineArgs(name ...string)
```
InlineArgs adds the specified flags to the argument map. It retrieves flags by
name and appends them to E.argMap.

#### func (*EFlagSet) IsSet

```go
func (s *EFlagSet) IsSet(name string) bool
```
IsSet reports whether a named flag has been set.

#### func (*EFlagSet) Multi

```go
func (E *EFlagSet) Multi(name string, value string, usage string) *[]string
```
Multi defines a multi-valued flag with the given name, initial value, and usage
string.

#### func (*EFlagSet) MultiVar

```go
func (E *EFlagSet) MultiVar(p *[]string, name string, value string, usage string)
```
MultiVar defines a multi-valued flag with the given name, initial value, and
usage string.

#### func (*EFlagSet) Order

```go
func (s *EFlagSet) Order(name ...string)
```
Order sets the order in which flags are processed. It takes a variable number of
flag names as input.

#### func (*EFlagSet) Parse

```go
func (s *EFlagSet) Parse(args []string) (err error)
```
Parse parses the command-line arguments and sets the corresponding flags.

#### func (*EFlagSet) PrintDefaults

```go
func (s *EFlagSet) PrintDefaults()
```
PrintDefaults prints the default values and usage information for all defined
flags. It formats the output in a human-readable format, including aliases and
default values.

#### func (*EFlagSet) ResolveAlias

```go
func (s *EFlagSet) ResolveAlias(name string) string
```
ResolveAlias resolves an alias for a given flag name. It returns the original
name if no alias is found.

#### func (*EFlagSet) SetOutput

```go
func (s *EFlagSet) SetOutput(output io.Writer)
```
SetOutput sets the output writer for the flag set.

#### func (*EFlagSet) Shorten

```go
func (s *EFlagSet) Shorten(name string, ch rune)
```
Shorten creates a short alias for a long flag name. It registers the alias in
the flag set and creates a reverse lookup.

#### func (*EFlagSet) SyntaxName

```go
func (E *EFlagSet) SyntaxName(name string)
```
SyntaxName sets the syntax name for the flag set.

#### func (*EFlagSet) VisitAll

```go
func (s *EFlagSet) VisitAll(fn func(*Flag))
```
VisitAll iterates over all defined flags and applies the given function to each.
Flags are processed in the order specified by the Order method, with any
remaining flags appended afterward.

#### type ErrorHandling

```go
type ErrorHandling int
```

ErrorHandling defines how errors are handled during flag parsing. It allows
control over whether parsing continues after an error or if it stops
immediately.

```go
const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
	ReturnErrorOnly
)
```
ErrorHandling defines strategies for handling errors. ContinueOnError continues
execution after an error. ExitOnError terminates the program on error.
PanicOnError causes a panic on error. ReturnErrorOnly returns the error without
further action.

#### type Flag

```go
type Flag = flag.Flag
```

Flag is an alias for the flag.Flag type. It represents a single command-line
flag.
