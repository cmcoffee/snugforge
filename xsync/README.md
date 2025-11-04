# xsync
--
    import "github.com/cmcoffee/snugforge/xsync"

LimitGroup is a sync.WaitGroup combined with a limiter, to limit how many
threads are created.

## Usage

#### type BitFlag

```go
type BitFlag uint64
```

Atomic BitFlag

#### func (*BitFlag) Has

```go
func (B *BitFlag) Has(flag uint64) bool
```
Check if flag is set

#### func (*BitFlag) Set

```go
func (B *BitFlag) Set(flag uint64) bool
```
Set BitFlag

#### func (*BitFlag) Switch

```go
func (B *BitFlag) Switch(flags ...uint64) uint64
```
Perform lookup of multiple flags, return index of first match or 0 if none

#### func (*BitFlag) Unset

```go
func (B *BitFlag) Unset(flag uint64) bool
```
Unset BitFlag

#### type LimitGroup

```go
type LimitGroup interface {
	Add(n int)
	Try() bool
	Done()
	Wait()
}
```


#### func  NewLimitGroup

```go
func NewLimitGroup(max int) LimitGroup
```
