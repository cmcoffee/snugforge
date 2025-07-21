# nfo
--
    import "github.com/cmcoffee/snugforge/nfo"

nfo package provides logging and output capabilities, including local log files
with rotation and simply output to termianl.

## Usage

```go
const (
	INFO   = 1 << iota // Log Information
	ERROR              // Log Errors
	WARN               // Log Warning
	NOTICE             // Log Notices
	DEBUG              // Debug Logging
	TRACE              // Trace Logging
	FATAL              // Fatal Logging
	AUX                // Auxiliary Log
	AUX2               // Auxiliary Log
	AUX3               // Auxiliary Log
	AUX4               // Auxiliary Log

)
```

```go
const (
	STD = INFO | ERROR | WARN | NOTICE | FATAL | AUX | AUX2 | AUX3 | AUX4
	ALL = INFO | ERROR | WARN | NOTICE | FATAL | AUX | AUX2 | AUX3 | AUX4 | DEBUG | TRACE
)
```
Standard Loggers, minus debug and trace.

```go
const (
	LeftToRight        = 1 << iota // Display progress bar left to right. (Default Behavior)
	RightToLeft                    // Display progress bar right to left.
	NoRate                         // Do not show transfer rate, left to right.
	MaxWidth                       // Scale width to maximum.
	ProgressBarSummary             // Maintain progress bar when transfer complete.
	NoSummary                      // Do not log a summary after completion.

)
```
LeftToRight displays the progress bar from left to right. RightToLeft displays
the progress bar from right to left. NoRate prevents the display of the transfer
rate. MaxWidth scales the progress bar width to maximum. ProgressBarSummary
maintains the progress bar after completion. NoSummary suppresses the summary
log after completion. internal is for internal use only. trans_active indicates
an active transfer. trans_closed indicates a closed transfer. trans_complete
indicates a completed transfer. trans_error indicates a transfer error.

```go
var (
	FatalOnFileError   = true // Fatal on log file or file rotation errors.
	FatalOnExportError = true // Fatal on export/syslog error.
	Animations         = true // Enable/Disable Flash Output

)
```

```go
var None dummyWriter
```
None represents a no-op io.Writer, discarding all writes.

```go
var PleaseWait = new(loading)
```
PleaseWait is a global variable representing the loading indicator.

#### func  Aux

```go
func Aux(vars ...interface{})
```
Aux logs an auxiliary message.

#### func  Aux2

```go
func Aux2(vars ...interface{})
```
Aux2 logs an auxiliary message.

#### func  Aux3

```go
func Aux3(vars ...interface{})
```
Aux3 is an auxiliary logging function.

#### func  Aux4

```go
func Aux4(vars ...interface{})
```
Aux4 logs an auxiliary message.

#### func  BlockShutdown

```go
func BlockShutdown()
```
BlockShutdown increments the WaitGroup counter, blocking shutdown until
Counter() becomes zero.

#### func  ConfirmDefault

```go
func ConfirmDefault(prompt string, default_answer bool) bool
```
Confirms a default answer to a boolean question from the user. Prompts the user
for confirmation with a default answer (Y/n or y/N).

#### func  Debug

```go
func Debug(vars ...interface{})
```
Debug logs debug-level messages.

#### func  Defer

```go
func Defer(closer interface{}) func() error
```
Defer registers a function to be called when all deferred functions have
returned. It returns a function that, when called, executes the registered
function.

#### func  DisableExport

```go
func DisableExport(flag uint32)
```
DisableExport disables specific exports using a bit flag. It atomically modifies
the enabled_exports bitmask.

#### func  EnableExport

```go
func EnableExport(flag uint32)
```
EnableExport enables specific export flags. It uses a mutex to ensure concurrent
safety.

#### func  Err

```go
func Err(vars ...interface{})
```
Err logs an error message.

#### func  Exit

```go
func Exit(exit_code int)
```
Exit terminates the program with the given exit code.

#### func  Fatal

```go
func Fatal(vars ...interface{})
```
Fatal terminates the program after logging a fatal error.

#### func  Flash

```go
func Flash(vars ...interface{})
```

#### func  GetConfirm

```go
func GetConfirm(prompt string) bool
```
GetConfirm prompts the user with a message and returns true if they enter "y" or
"yes", and false if they enter "n" or "no".

#### func  GetFile

```go
func GetFile(flag uint32) io.Writer
```
GetFile returns the file writer associated with the given flag.

#### func  GetInput

```go
func GetInput(prompt string) string
```
GetInput prompts the user for input and returns the cleaned string. It reads
from standard input until a newline character is encountered.

#### func  GetOutput

```go
func GetOutput(flag uint32) io.Writer
```
GetOutput returns the io.Writer associated with the given flag.

#### func  GetSecret

```go
func GetSecret(prompt string) string
```
GetSecret prompts the user for a secret string. It disables terminal echoing
while reading input.

#### func  HideTS

```go
func HideTS(flag ...uint32)
```
HideTS disables timestamp output for all log levels. It accepts optional flags
to specify which log levels to disable timestamps for; if no flags are provided,
timestamps are disabled for all levels.

#### func  HookSyslog

```go
func HookSyslog(syslog_writer SyslogWriter)
```
HookSyslog replaces the default syslog writer with a custom one. It's protected
by a mutex for concurrent safety.

#### func  HumanSize

```go
func HumanSize(bytes int64) string
```
HumanSize returns a human-readable string representation of a size in bytes.

#### func  Itoa

```go
func Itoa(buf *[]byte, i int, wid int)
```
Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid
zero-padding.

#### func  LTZ

```go
func LTZ()
```
LTZ sets the timezone to local time using a mutex lock.

#### func  Log

```go
func Log(vars ...interface{})
```
Log outputs a log message with default INFO level.

#### func  LogFile

```go
func LogFile(filename string, max_size_mb uint, max_rotation uint) (io.Writer, error)
```
LogFile opens or creates a log file, optionally rotating it. It returns an
io.Writer for writing to the log file and an error, if any.

#### func  NeedAnswer

```go
func NeedAnswer(prompt string, request func(prompt string) string) (output string)
```
NeedAnswer repeatedly requests an answer from a function until a non-empty
string is returned.

#### func  Notice

```go
func Notice(vars ...interface{})
```
Notice logs a notice message.

#### func  PressEnter

```go
func PressEnter(prompt string)
```
PressEnter prints a prompt and waits for the user to press Enter. It masks the
input to prevent it from being displayed on the screen.

#### func  SetFile

```go
func SetFile(flag uint32, input io.Writer)
```
SetFile sets the file writer for logging. It updates the logger with the
provided io.WriteCloser based on the given flag.

#### func  SetOutput

```go
func SetOutput(flag uint32, w io.Writer)
```
SetOutput sets the io.Writer for a specific output flag. It updates the logger
to use the provided writer for the flag.

#### func  SetPrefix

```go
func SetPrefix(logger uint32, prefix_str string)
```

#### func  SetSignals

```go
func SetSignals(sig ...os.Signal)
```
SetSignals sets the signals to be notified on. It stops any existing signal
notifications and registers the provided signals.

#### func  SetTZ

```go
func SetTZ(location string) (err error)
```

#### func  ShowTS

```go
func ShowTS(flag ...uint32)
```
ShowTS enables or disables timestamp logging. If no flag is provided, it
defaults to ALL flags.

#### func  ShutdownInProgress

```go
func ShutdownInProgress() bool
```
ShutdownInProgress reports whether a shutdown is in progress. It checks the
value of the fatal_triggered atomic integer.

#### func  SignalCallback

```go
func SignalCallback(signal os.Signal, callback func() (continue_shutdown bool))
```
SignalCallback registers a callback function to be executed when a specific OS
signal is received.

#### func  Stderr

```go
func Stderr(vars ...interface{})
```
Stderr writes log messages to standard error.

#### func  Stdout

```go
func Stdout(vars ...interface{})
```
Stdout outputs variables to standard output.

#### func  Stringer

```go
func Stringer(vars ...interface{}) string
```

#### func  Trace

```go
func Trace(vars ...interface{})
```
Trace logs trace level messages.

#### func  UTC

```go
func UTC()
```
/ UTC sets the timezone to UTC.

#### func  UnblockShutdown

```go
func UnblockShutdown()
```
UnblockShutdown signals the completion of a shutdown process. It decrements a
WaitGroup counter, potentially unblocking a waiting shutdown routine.

#### func  UnhookSyslog

```go
func UnhookSyslog()
```
UnhookSyslog resets the syslog writer to its default state. It removes any
custom syslog writer that was previously set.

#### func  Warn

```go
func Warn(vars ...interface{})
```
Warn logs a warning message.

#### type ProgressBar

```go
type ProgressBar interface {
	Add(num int) // Add num to progress bar.
	Set(num int) // Set num of progress bar.
	Done()       // Mark progress bar as complete.
}
```

ProgressBar interface for tracking progress. Defines methods to add to, set, and
mark progress as complete.

#### func  NewProgressBar

```go
func NewProgressBar(name string, max int) ProgressBar
```
NewProgressBar creates a new progress bar with the given name and maximum value.

#### type ReadSeekCloser

```go
type ReadSeekCloser interface {
	Seek(offset int64, whence int) (int64, error)
	Read(p []byte) (n int, err error)
	Close() error
}
```

ReadSeekCloser is an interface that wraps the basic Read, Seek, and Close
methods. It allows reading from and seeking within a data source, and then
closing the source when finished.

#### func  NopSeeker

```go
func NopSeeker(input io.ReadCloser) ReadSeekCloser
```
NopSeeker returns a ReadSeekCloser that does not perform seeking. It wraps the
provided io.ReadCloser and always returns 0, nil from Seek.

#### func  TransferCounter

```go
func TransferCounter(input ReadSeekCloser, counter func(int)) ReadSeekCloser
```
TransferCounter wraps a ReadSeekCloser and counts the number of bytes read. It
returns a new ReadSeekCloser that calls the provided counter function after each
Read operation with the number of bytes read.

#### func  TransferMonitor

```go
func TransferMonitor(name string, total_size int64, flag int, source ReadSeekCloser, optional_prefix ...string) ReadSeekCloser
```
TransferMonitor creates a transfer monitor and starts displaying transfer
progress. It accepts the name of the transfer, the total size, flags, the
source, and an optional prefix. It returns a ReadSeekCloser that wraps the
original source and allows monitoring of the transfer.

#### type SyslogWriter

```go
type SyslogWriter interface {
	Alert(string) error
	Crit(string) error
	Debug(string) error
	Emerg(string) error
	Err(string) error
	Info(string) error
	Notice(string) error
	Warning(string) error
}
```

SyslogWriter defines an interface for writing syslog messages. It provides
methods for different severity levels.
