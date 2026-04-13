# xpect
--
    import "github.com/cmcoffee/snugforge/xpect"

Package xpect provides expect-like automation for interactive command-line
programs.

It allows spawning a process, sending input, and waiting for expected output
patterns with configurable timeouts — similar to the Unix expect tool.

## Usage

```go
const DefaultTimeout = 30 * time.Second
```
Default timeout for expect operations when none is specified.

```go
var ErrClosed = errors.New("Session is closed.")
```
ErrClosed indicates that the session has already been closed.

```go
var ErrTimeout = errors.New("Timeout reached while waiting for expected output.")
```
ErrTimeout indicates that an expect operation timed out waiting for a match.

#### type Match

```go
type Match struct {
	// Before contains all output received before the match.
	Before string

	// Full contains the full text matched by the pattern.
	Full string

	// Groups contains any capture group matches from the pattern.
	Groups []string
}
```

Match holds the result of a successful expect operation.

#### type Session

```go
type Session struct {
	// Log, when set, receives a copy of all process output as it is read.
	// Set to os.Stdout to watch the session in real time, or any io.Writer for logging.
	Log io.Writer

	// SendMask, when non-empty, replaces the logged representation of the next Send or
	// SendLine call. Useful for masking passwords. Resets to empty after one use.
	SendMask string

	// SendLog, when true, logs all Send and SendLine calls to Log.
	// Masked by SendMask when set.
	SendLog bool
}
```

Session manages an interactive process and provides expect-style operations.

#### func  Command

```go
func Command(name string, args ...string) (*Session, error)
```
Command starts a new interactive session with the given command and arguments.
The process is started immediately. Returns an error if the process fails to
start.

#### func (*Session) Clear

```go
func (s *Session) Clear()
```
Clear discards any unmatched output currently in the buffer.

#### func (*Session) Close

```go
func (s *Session) Close() error
```
Close closes the session by closing stdin and killing the process.

#### func (*Session) Expect

```go
func (s *Session) Expect(pattern string) (Match, error)
```
Expect waits for the process output to match the given regular expression
pattern. It uses the session's default timeout.

#### func (*Session) ExpectEOF

```go
func (s *Session) ExpectEOF() error
```
ExpectEOF waits for the process to close its output stream. It uses the
session's default timeout.

#### func (*Session) ExpectEOFTimeout

```go
func (s *Session) ExpectEOFTimeout(timeout time.Duration) error
```
ExpectEOFTimeout waits for the process to close its output stream within the
specified timeout duration.

#### func (*Session) ExpectTimeout

```go
func (s *Session) ExpectTimeout(pattern string, timeout time.Duration) (Match, error)
```
ExpectTimeout waits for the process output to match the given regular expression
pattern within the specified timeout duration.

#### func (*Session) Interact

```go
func (s *Session) Interact(r io.Reader, w io.Writer) error
```
Interact hands control to the user for live two-way interaction with the
process. User input from r is forwarded to the process, and process output is
written to w. Blocks until the process exits or the input reader returns an
error (e.g. EOF). After Interact returns, scripted Expect/Send calls can resume.

#### func (*Session) InteractUntil

```go
func (s *Session) InteractUntil(r io.Reader, w io.Writer, pattern string) (Match, error)
```
InteractUntil hands control to the user like Interact, but returns control to
the script when the process output matches the given regular expression pattern.
The matched output is consumed from the buffer, and the Match result is
returned.

#### func (*Session) Output

```go
func (s *Session) Output() string
```
Output returns any unmatched output currently in the buffer.

#### func (*Session) Send

```go
func (s *Session) Send(input string) error
```
Send writes the given string to the process's standard input.

#### func (*Session) SendLine

```go
func (s *Session) SendLine(input string) error
```
SendLine writes the given string followed by a newline to the process's standard
input.

#### func (*Session) SetTimeout

```go
func (s *Session) SetTimeout(d time.Duration)
```
SetTimeout sets the default timeout for expect operations.

#### func (*Session) Wait

```go
func (s *Session) Wait() error
```
Wait waits for the process to exit and returns its exit status.
