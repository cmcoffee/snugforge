# logtime
--
    import "github.com/cmcoffee/snugforge/logtime"


## Usage

```go
const (
	// ISO 8601 variants
	DateTimeMicro  = "2006-01-02T15:04:05.000000Z07:00" // [2026-02-05T00:46:12.878593+00:00]
	DateTimeMilliT = "2006-01-02T15:04:05.000Z07:00"    // 2026-03-05T09:01:20.308+00:00
	ISO8601        = "2006-01-02T15:04:05Z07:00"        // 2026-03-05T09:01:20+00:00
	RFC3339Nano    = time.RFC3339Nano                   // 2026-03-05T09:01:20.999999999Z07:00

	// Space-separated date time variants
	DateTimeMicroTZ = "2006-01-02 15:04:05.000000 Z07:00" // 2026-03-05 03:28:19.308000 +00:00
	DateTimeMilliTZ = "2006-01-02 15:04:05.000 Z07:00"    // 2026-03-05 03:28:19.308 +00:00
	DateTimeTZ      = "2006-01-02 15:04:05 Z07:00"        // 2026-03-05 03:28:19 +00:00
	DateTimeMST     = "2006-01-02 15:04:05 MST"           // 2026-03-05 03:28:19 UTC
	DateTimeMicro2  = "2006-01-02 15:04:05.000000"        // 2026-03-05 03:28:19.308000
	DateTimeMilli   = "2006-01-02 15:04:05.000"           // 2026-03-05 03:28:19.308
	DateTime        = "2006-01-02 15:04:05"               // 2026-03-05 03:28:19

	// Common/human-readable variants
	CommonDateTime  = "02-Jan-2006 15:04:05 MST" // 23-Dec-2025 19:57:43 UTC
	CommonDateTime2 = "02/Jan/2006 15:04:05 MST" // 23/Dec/2025 19:57:43 UTC

	// Apache/CLF
	ApacheCLF = "02/Jan/2006:15:04:05 -0700" // 05/Mar/2026:03:28:19 +0000

	// ANSIC / Unix
	ANSIC     = "Mon Jan  2 15:04:05 2006"     // Thu Mar  5 03:28:19 2026
	ANSICDate = "Mon Jan 2 15:04:05 2006"      // Thu Mar 5 03:28:19 2026
	UnixDate  = "Mon Jan  2 15:04:05 MST 2006" // Thu Mar  5 03:28:19 UTC 2026

	// RFC variants
	RFC1123  = time.RFC1123  // Mon, 02 Jan 2006 15:04:05 MST
	RFC1123Z = time.RFC1123Z // Mon, 02 Jan 2006 15:04:05 -0700
	RFC822   = time.RFC822   // 02 Jan 06 15:04 MST
	RFC822Z  = time.RFC822Z  // 02 Jan 06 15:04 -0700
	RFC850   = time.RFC850   // Monday, 02-Jan-06 15:04:05 MST

	// Syslog (no year)
	Syslog     = "Jan  2 15:04:05" // Mar  5 03:28:19
	SyslogDate = "Jan 2 15:04:05"  // Mar 5 03:28:19
)
```
Format constants for common log timestamp layouts.

```go
var Formats = []string{

	DateTimeMicro,
	DateTimeMilliT,
	RFC3339Nano,
	ISO8601,

	DateTimeMicroTZ,
	DateTimeMilliTZ,
	DateTimeTZ,
	DateTimeMST,
	DateTimeMicro2,
	DateTimeMilli,
	DateTime,

	CommonDateTime,
	CommonDateTime2,

	ApacheCLF,

	UnixDate,
	ANSIC,
	ANSICDate,

	RFC1123,
	RFC1123Z,
	RFC850,
	RFC822,
	RFC822Z,

	Syslog,
	SyslogDate,
}
```
Formats is the list of timestamp layouts attempted by Parse, in order. More
specific formats are listed first to avoid false matches. Append to this slice
or use Register to add additional formats.

#### func  MustParse

```go
func MustParse(value string) time.Time
```
MustParse is like Parse but panics on failure.

#### func  Parse

```go
func Parse(value string) (time.Time, error)
```
Parse attempts to parse the timestamp string against each layout in Formats.
Surrounding brackets are stripped before parsing. Returns the first successful
parse or an error if none match.

#### func  Register

```go
func Register(layouts ...string)
```
Register adds one or more time.Parse layout strings to Formats.

#### type Scanner

```go
type Scanner struct {
}
```

Scanner reads log entries from an io.Reader, grouping continuation lines (lines
without a timestamp) with the preceding timestamped line, and filtering to
entries whose timestamp falls within a start/stop window.

#### func  NewScanner

```go
func NewScanner(example string, start, stop time.Time, r io.Reader) (*Scanner, error)
```
NewScanner creates a Scanner that reads log entries from r. The example can be a
bare timestamp or a full log line; the timestamp layout and prefix length are
auto-detected from the leading tokens. Only entries with timestamps in [start,
stop] are returned.

#### func (*Scanner) Err

```go
func (s *Scanner) Err() error
```
Err returns the first non-EOF error encountered by the Scanner.

#### func (*Scanner) Scan

```go
func (s *Scanner) Scan() bool
```
Scan advances to the next log entry within the time window. It returns false
when there are no more entries or an error occurs.

#### func (*Scanner) Text

```go
func (s *Scanner) Text() string
```
Text returns the full text of the current log entry, including continuation
lines.

#### func (*Scanner) Time

```go
func (s *Scanner) Time() time.Time
```
Time returns the parsed timestamp of the current log entry.
