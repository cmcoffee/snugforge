# cfg
--
    import "github.com/cmcoffee/snugforge/cfg"

Package 'cfg' provides functions for reading and writing configuration files and
their coresponding string values.

    Ignores '#' as comments, ','s denote multiple values.

    # Example config file.
    [section]
    key = value
    key2 = value1, value2
    key3 = value1,
           value2,
           value3

    [section2]
    key = value1,
          value2,
          value3

## Usage

#### type Store

```go
type Store struct {
}
```

Store represents a configuration store.

#### func (*Store) Defaults

```go
func (s *Store) Defaults(input string) (err error)
```
Defaults parses the given string as configuration, updating the store. It does
not overwrite existing values, only sets those not already present.

#### func (*Store) Exists

```go
func (s *Store) Exists(input ...string) (found bool)
```
Returns true if section or section and key exists.

#### func (*Store) File

```go
func (s *Store) File(file string) (err error)
```
File opens the given file, parses it as a configuration, and stores it.

#### func (*Store) Get

```go
func (s *Store) Get(section, key string) string
```
Get returns the value associated with the given section and key. Returns an
empty string if the section or key does not exist.

#### func (*Store) GetBool

```go
func (s *Store) GetBool(section, key string) (output bool)
```
GetBool returns the boolean value associated with the given section and key.
Returns false if the section or key does not exist, or if the value is not "yes"
or "true".

#### func (*Store) GetFloat

```go
func (s *Store) GetFloat(section, key string) (output float64)
```
GetFloat returns the float64 value associated with the given section and key.
Returns 0.0 if the section or key does not exist.

#### func (*Store) GetInt

```go
func (s *Store) GetInt(section, key string) (output int64)
```
GetInt returns the integer value from section key provided, (output, bool, err
error)

#### func (*Store) GetUint

```go
func (s *Store) GetUint(section, key string) (output uint64)
```
GetUint returns the value associated with the given section and key as a uint64.
Returns 0 if the section or key does not exist, or if parsing fails.

#### func (*Store) Keys

```go
func (s *Store) Keys(section string) (out []string)
```
Returns keys of section specified.

#### func (*Store) MGet

```go
func (s *Store) MGet(section, key string) []string
```
MGet returns a slice of strings associated with the given section and key.
Returns an empty slice if the section or key does not exist.

#### func (*Store) Parse

```go
func (s *Store) Parse(input string) (err error)
```
Parse parses a string as configuration data. It calls the internal config_parser
to handle the parsing process.

#### func (*Store) SGet

```go
func (s *Store) SGet(section, key string) string
```
SGet returns the value associated with the given section and key. Returns an
empty string if the section or key does not exist. If multiple values exist,
they are joined with a comma and space.

#### func (*Store) Sanitize

```go
func (s *Store) Sanitize(section string, keys []string) (err error)
```
Sanitize checks if the specified section and keys exist in the configuration. It
returns an error if the section doesn't exist or if any of the keys are missing.

#### func (*Store) Save

```go
func (s *Store) Save(sections ...string) error
```
Save persists the store's data to disk, optionally for specific sections.

#### func (*Store) Sections

```go
func (s *Store) Sections() (out []string)
```
Returns array of all sections in config file.

#### func (*Store) Set

```go
func (s *Store) Set(section, key string, value ...interface{}) (err error)
```
Sets key = values under [section], updates Store and saves to file.

#### func (*Store) TrimSave

```go
func (s *Store) TrimSave(sections ...string) error
```
TrimSave removes unused sections before saving the store. It calls the save
method with the trim flag set to true and the provided sections.

#### func (*Store) Unset

```go
func (s *Store) Unset(input ...string)
```
Unsets a specified key, or specified section. If section is empty, section is
removed.
