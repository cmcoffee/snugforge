# options
--
    import "github.com/cmcoffee/snugforge/options"


## Usage

#### type Options

```go
type Options struct {
}
```

Options represents a menu of configurable options.

#### func  NewOptions

```go
func NewOptions(header, footer string, exit_char rune) *Options
```
NewOptions creates a new Options instance with the given header, footer, and
exit character.

#### func (*Options) Bool

```go
func (O *Options) Bool(desc string, value bool) *bool
```
Bool registers a boolean option with the given description and default value. It
returns a pointer to the boolean variable holding the value.

#### func (*Options) BoolVar

```go
func (O *Options) BoolVar(p *bool, desc string, value bool)
```
BoolVar sets a boolean option with the given description and default value. It
registers the option with the Options menu.

#### func (*Options) Func

```go
func (O *Options) Func(desc string, value func() bool)
```
Registers a function as an option that, when selected, executes the function and
returns a boolean.

#### func (*Options) Int

```go
func (O *Options) Int(desc string, value int, help string, min, max int) *int
```
Int registers an integer option with the given description, default value, help
string, and range. It returns a pointer to the integer variable holding the
value.

#### func (*Options) IntVar

```go
func (O *Options) IntVar(p *int, desc string, value int, help string, min, max int)
```
IntVar sets an integer option with the given description, default value, help
string, and range. It registers the option with the Options menu.

#### func (*Options) Options

```go
func (O *Options) Options(desc string, value *Options, separate_last bool)
```
Options registers a nested Options menu as an option. It allows for hierarchical
configuration.

#### func (*Options) Register

```go
func (T *Options) Register(input Value)
```
Register adds a configurable option to the options menu.

#### func (*Options) Secret

```go
func (O *Options) Secret(desc string, value string, help string) *string
```
Secret registers a string option with masked display. It returns a pointer to
the string variable holding the value.

#### func (*Options) SecretVar

```go
func (O *Options) SecretVar(p *string, desc string, value string, help string)
```
SecretVar registers a string option with masked display. It returns a pointer to
the string variable holding the value.

#### func (*Options) Select

```go
func (T *Options) Select(separate_last bool) (changed bool)
```
Select displays a menu of configurable options and allows the user to make a
selection. It returns true if a change was made, false otherwise.

#### func (*Options) String

```go
func (O *Options) String(desc string, value string, help string, mask_value bool) *string
```
String defines a string menu option displaying with specified desc in menu,
default value, and help string. The return value is the address of an string
variable that stores the value of the option.

#### func (*Options) StringVar

```go
func (O *Options) StringVar(p *string, desc string, value string, help string)
```
StringVar sets a string option with the given description, default value, and
help string. It registers the option with the Options menu.

#### type Value

```go
type Value interface {
	Set() bool
	Get() interface{}
	String() string
}
```

Value defines an interface for configurable options. It allows setting, getting,
and string representation of a value.
