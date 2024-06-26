# options
--
    import "github.com/cmcoffee/snugforge/options"


## Usage

#### type Options

```go
type Options struct {
}
```


#### func  NewOptions

```go
func NewOptions(header, footer string, exit_char rune) *Options
```
Creates new Options Menu

#### func (*Options) Bool

```go
func (O *Options) Bool(desc string, value bool) *bool
```
Bool defines an int menu option displaying with specified desc in menu, default
value, and help string. The return value is the address of an bool variable that
stores the value of the option.

#### func (*Options) BoolVar

```go
func (O *Options) BoolVar(p *bool, desc string, value bool)
```
BoolVar defines a bool menu option displaying with specified desc in menu,
default value, and help string. The argument p points to a bool variable in
which to store the value of the option.

#### func (*Options) Func

```go
func (O *Options) Func(desc string, value func() bool)
```
Func defined a function within the option menu, the function should return a
bool variable telling the Options menu if a change has occurred.

#### func (*Options) Int

```go
func (O *Options) Int(desc string, value int, help string, min, max int) *int
```
Int defines an int menu option displaying with specified desc in menu, default
value, and help string. The return value is the address of an int variable that
stores the value of the option.

#### func (*Options) IntVar

```go
func (O *Options) IntVar(p *int, desc string, value int, help string, min, max int)
```

#### func (*Options) Options

```go
func (O *Options) Options(desc string, value *Options, separate_last bool)
```
Option defines an nested Options menu option displaying with specified desc in
menu, separate_last will separate the last menu option within the sub Options
when selected.

#### func (*Options) Register

```go
func (T *Options) Register(input Value)
```
Registers an Value with Options Menu

#### func (*Options) Secret

```go
func (O *Options) Secret(desc string, value string, help string) *string
```
Secret defines an string menu option displaying with specified desc in menu,
default value, and help string. The return value is the address of an string
variable that stores the value of the option.

#### func (*Options) SecretVar

```go
func (O *Options) SecretVar(p *string, desc string, value string, help string)
```
SecretVar defines a string with specified name, value is displayed as masked,
default value and usage string. The argument p points to a string variable in
which to store the value of the flag.

#### func (*Options) Select

```go
func (T *Options) Select(separate_last bool) (changed bool)
```
Show Options Menu, if separate_last = true, the last menu item will be dropped
one line, and it's select number will be 0, seperating it from the rest.

#### func (*Options) String

```go
func (O *Options) String(desc string, value string, help string, mask_value bool) *string
```
String defines an string menu option displaying with specified desc in menu,
default value, and help string. The return value is the address of an string
variable that stores the value of the option.

#### func (*Options) StringVar

```go
func (O *Options) StringVar(p *string, desc string, value string, help string)
```
StringVar defines a string flag with specified name, default value, and usage
string. The argument p points to a string variable in which to store the value
of the flag.

#### type Value

```go
type Value interface {
	Set() bool
	Get() interface{}
	String() string
}
```

Options Value
