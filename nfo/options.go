package nfo

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/cmcoffee/snugforge/xsync"
)

// Options represents a menu of configurable options.
type Options struct {
	header    string
	footer    string
	exit_char rune
	flags     xsync.BitFlag
	config    []Value
}

// Value defines an interface for configurable options.
// It allows setting, getting, and string representation of a value.
type Value interface {
	Set() bool
	Get() interface{}
	String() string
}

// NewOptions creates a new Options instance with the given header, footer, and exit character.
func NewOptions(header, footer string, exit_char rune) *Options {
	return &Options{
		header:    header,
		footer:    footer,
		exit_char: exit_char,
		flags:     0,
		config:    make([]Value, 0),
	}
}

// Register adds a configurable option to the options menu.
func (T *Options) Register(input Value) {
	T.config = append(T.config, input)
}

// Select displays a menu of configurable options and allows the user to make a selection.
// It returns true if a change was made, false otherwise.
func (T *Options) Select(separate_last bool) (changed bool) {
	var text_buffer bytes.Buffer
	txt := tabwriter.NewWriter(&text_buffer, 1, 8, 1, ' ', 0)

	show_banner := func() {
		if len(T.header) > 0 {
			text_buffer.Reset()
			fmt.Fprintf(txt, T.header)
			fmt.Fprintf(txt, "\n\n")
			txt.Flush()

			Stdout(text_buffer.String())
		}
	}

	show_banner()

	for {
		text_buffer.Reset()
		config_map := make(map[int]Value)
		config_len := len(T.config) - 1

		for i := 0; i <= config_len; i++ {
			if i == config_len && config_len > 0 && separate_last {
				config_map[0] = T.config[config_len]
				fmt.Fprintf(txt, "\t\n")
				fmt.Fprintf(txt, " [0] %s\n", T.config[config_len].String())
				break
			}
			config_map[i+1] = T.config[i]
			fmt.Fprintf(txt, " [%d] %s\n", i+1, T.config[i].String())
		}

		fmt.Fprintf(txt, "\n%s: ", T.footer)
		txt.Flush()

		input := GetInput(text_buffer.String())
		if strings.ToLower(input) == strings.ToLower(string(T.exit_char)) {
			return
		} else {
			sel, err := strconv.Atoi(input)
			if err != nil {
				Stdout("\n[ERROR] Unrecognized Selection: '%s'\n\n", input)
				continue
			} else {
				if v, ok := config_map[sel]; ok {
					changed = v.Set()
					switch v.(type) {
					case *funcValue:
						Stdout("\n")
						show_banner()
					case *optionsValue:
						Stdout("\n")
						show_banner()
					default:
						Stdout("\n")
					}
					continue
				}
				Stdout("\n[ERROR] Unrecognized Selection: '%s'\n\n", input)
			}
		}
	}
}

// showVar returns a string, masking it with '*' if 'mask' is true.
// If the input string is empty, it returns a default message.
func showVar(input string, mask bool) string {
	hide_value := func(input string) string {
		var str []rune
		for range input {
			str = append(str, '*')
		}
		return string(str)
	}

	if len(input) == 0 {
		return "*** UNCONFIGURED ***"
	} else {
		if !mask {
			return input
		} else {
			return hide_value(input)
		}
	}
}

// String defines a string menu option displaying with specified
// desc in menu, default value, and help string. The return value
// is the address of an string variable that stores the value of
// the option.
func (O *Options) String(desc string, value string, help string, mask_value bool) *string {
	new_var := &stringValue{
		desc:  desc,
		value: &value,
		help:  help,
		mask:  mask_value,
	}
	O.Register(new_var)
	return &value
}

// Secret registers a string option with masked display.
// It returns a pointer to the string variable holding the value.
func (O *Options) Secret(desc string, value string, help string) *string {
	new_var := &stringValue{
		desc:  desc,
		value: &value,
		help:  help,
		mask:  true,
	}
	O.Register(new_var)
	return &value
}

// SecretVar registers a string option with masked display.
// It returns a pointer to the string variable holding the value.
func (O *Options) SecretVar(p *string, desc string, value string, help string) {
	*p = value
	O.Register(&stringValue{
		desc:  desc,
		value: p,
		help:  help,
		mask:  true,
	})
	return
}

// StringVar sets a string option with the given description, default value, and help string.
// It registers the option with the Options menu.
func (O *Options) StringVar(p *string, desc string, value string, help string) {
	*p = value
	O.Register(&stringValue{
		desc:  desc,
		value: p,
		help:  help,
		mask:  false,
	})
	return
}

// Bool registers a boolean option with the given description and default value.
// It returns a pointer to the boolean variable holding the value.
func (O *Options) Bool(desc string, value bool) *bool {
	new_var := &boolValue{
		desc:  desc,
		value: &value,
	}
	O.Register(new_var)
	return &value
}

// BoolVar sets a boolean option with the given description and default value.
// It registers the option with the Options menu.
func (O *Options) BoolVar(p *bool, desc string, value bool) {
	*p = value
	O.Register(&boolValue{
		desc:  desc,
		value: p,
	})
}

// Int registers an integer option with the given description, default value, help string, and range.
// It returns a pointer to the integer variable holding the value.
func (O *Options) Int(desc string, value int, help string, min, max int) *int {
	new_var := &intValue{
		desc:  desc,
		value: &value,
		help:  help,
		min:   min,
		max:   max,
	}
	O.Register(new_var)
	return &value
}

// IntVar sets an integer option with the given description, default value, help string, and range.
// It registers the option with the Options menu.
func (O *Options) IntVar(p *int, desc string, value int, help string, min, max int) {
	*p = value
	O.Register(&intValue{
		desc:  desc,
		value: p,
		min:   min,
		max:   max,
		help:  help,
	})
}

// Options registers a nested Options menu as an option.
// It allows for hierarchical configuration.
func (O *Options) Options(desc string, value *Options, separate_last bool) {
	O.Register(&optionsValue{
		desc:          desc,
		value:         value,
		separate_last: separate_last,
	})
}

// Registers a function as an option that, when selected, executes the function and returns a boolean.
func (O *Options) Func(desc string, value func() bool) {
	O.Register(&funcValue{
		desc:  desc,
		value: value,
	})
}

// StringValue represents a string-based configuration option.
// It stores the description, a pointer to the string value,
// help text, and a flag indicating whether the value should
// be masked (e.g., for passwords).
type stringValue struct {
	desc  string
	value *string
	help  string
	mask  bool
}

// Set prompts the user for input and sets the string value.
// Returns true if input was provided, false otherwise.
func (S *stringValue) Set() bool {
	var input string
	if len(S.help) > 0 {
		input = GetInput(fmt.Sprintf("\n# %s\n--> %s: ", S.help, S.desc))
	} else {
		input = GetInput(fmt.Sprintf("\n--> %s: ", S.desc))
	}
	if len(input) > 0 {
		*S.value = input
		return true
	}
	return false
}

// String returns a string representation of the string value,
// including the description and the masked or unmasked value.
func (S *stringValue) String() string {
	return fmt.Sprintf("%s: \t%s", S.desc, showVar(*S.value, S.mask))
}

// Get returns the current string value.
func (S *stringValue) Get() interface{} {
	return S.value
}

// boolValue represents a boolean option with its description and value.
type boolValue struct {
	desc  string
	value *bool
}

// IsSet returns whether the value has been set.
func (B *boolValue) IsSet() bool {
	return true
}

// Set toggles the boolean value stored within the boolValue.
// It returns true if the toggle was successful, otherwise false.
func (B *boolValue) Set() bool {
	*B.value = *B.value == false
	return true
}

// Get returns the underlying bool value.
func (B *boolValue) Get() interface{} {
	return B.value
}

// String returns a string representation of the boolean value.
func (B *boolValue) String() string {
	var value_str string
	if *B.value {
		value_str = "yes"
	} else {
		value_str = "no"
	}
	return fmt.Sprintf("%s:\t%s", B.desc, value_str)
}

// intValue represents an integer option with description, help, value, min, max, and changed flag.
type intValue struct {
	desc    string
	help    string
	value   *int
	min     int
	max     int
	changed int
}

func (I *intValue) Set() bool {
	for {
		var input string
		if len(I.help) > 0 {
			input = GetInput(fmt.Sprintf("\n# %s\n--> %s (%d-%d): ", I.help, I.desc, I.min, I.max))
		} else {
			input = GetInput(fmt.Sprintf("--> %s (%d-%d): ", I.desc, I.min, I.max))
		}
		if len(input) > 0 {
			val, err := strconv.Atoi(input)
			if err != nil {
				Stdout("\n[ERROR] Value must be an integer between %d and %d.", I.min, I.max)
				continue
			}
			if val > I.max || val < I.min {
				Stdout("\n[ERROR] Value is outside of acceptable range of %d and %d.", I.min, I.max)
				continue
			}
			*I.value = val
			return true
		}
		return false
	}
}

// Get returns the current integer value.
func (I *intValue) Get() interface{} {
	return I.value
}

// IsSet always returns true.
func (I *intValue) IsSet() bool {
	return true
}

// String returns a string representation of the intValue.
func (I *intValue) String() string {
	return fmt.Sprintf("%s:\t%d", I.desc, *I.value)
}

// optionsValue represents a single option with its description,
// separation flag, and associated Options object.
type optionsValue struct {
	desc          string
	separate_last bool
	value         *Options
}

// Set displays the options menu and allows the user to make a selection.
// It returns true if a change was made, false otherwise.
func (O *optionsValue) Set() bool {
	Stdout("\n")
	return O.value.Select(O.separate_last)
}

// Get returns the value of the option.
func (O *optionsValue) Get() interface{} {
	return nil
}

// String returns the description of the option value.
func (O *optionsValue) String() string {
	return O.desc
}

// funcValue represents a function with a description.
// It stores a boolean function and a string describing it.
type funcValue struct {
	desc  string
	value func() bool
}

// String returns the description of the function.
func (f *funcValue) String() string {
	return fmt.Sprintf("%s\t", f.desc)
}

// Get returns the value.
func (f *funcValue) Get() interface{} {
	return nil
}

// Set executes the stored function and returns a boolean value.
func (f *funcValue) Set() bool {
	Stdout("\n")
	return f.value()
}
