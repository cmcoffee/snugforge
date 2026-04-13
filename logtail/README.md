# logtail
--
    import "github.com/cmcoffee/snugforge/logtail"

Package logtail provides continuous log file tailing with pattern-matched
callbacks.

It watches a log file for new content, parses timestamps using the logtime
package, and fires registered callbacks when lines match specified regular
expression patterns. Timestamp tracking provides a safety net against
reprocessing entries after file rotation or tailer restart.

## Usage

```go
const DefaultInterval = 250 * time.Millisecond
```
Default poll interval for checking new file content.

#### type Error

```go
type Error string
```

Error is a simple string error type.

#### func (Error) Error

```go
func (e Error) Error() string
```

#### type Match

```go
type Match struct {
	// Time is the parsed timestamp of the log entry.
	Time time.Time

	// Text is the log line with the timestamp prefix stripped.
	Text string

	// Full is the original complete log line including the timestamp.
	Full string

	// Groups contains any capture group matches from the pattern.
	Groups []string
}
```

Match holds the result of a pattern match against a log entry.

#### type Tailer

```go
type Tailer struct {
}
```

Tailer watches a log file and fires callbacks when patterns match new entries.

#### func  Open

```go
func Open(path string) *Tailer
```
Open creates a new Tailer for the given log file path. The file is not opened
until Run is called.

#### func (*Tailer) Close

```go
func (t *Tailer) Close()
```
Close stops the tailer. It is safe to call from any goroutine.

#### func (*Tailer) FromStart

```go
func (t *Tailer) FromStart()
```
FromStart configures the tailer to process existing file content instead of
seeking to the end on start.

#### func (*Tailer) MustOn

```go
func (t *Tailer) MustOn(pattern string, fn func(Match))
```
MustOn is like On but panics if the pattern fails to compile.

#### func (*Tailer) On

```go
func (t *Tailer) On(pattern string, fn func(Match)) error
```
On registers a callback that fires when a new log entry matches the given
regular expression pattern. The pattern is matched against the
timestamp-stripped text of each entry. Multiple rules may be registered and all
matching rules fire for each entry.

#### func (*Tailer) Run

```go
func (t *Tailer) Run() error
```
Run begins tailing the log file. It blocks until Close is called or an
unrecoverable error occurs. File rotation and truncation are handled
automatically by detecting size changes and re-opening the file.

#### func (*Tailer) SetInterval

```go
func (t *Tailer) SetInterval(d time.Duration)
```
SetInterval sets the poll interval for checking new file content.
