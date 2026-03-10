package logtime

import (
	"fmt"
	"strings"
	"time"
)

// Format constants for common log timestamp layouts.
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

// Formats is the list of timestamp layouts attempted by Parse, in order.
// More specific formats are listed first to avoid false matches.
// Append to this slice or use Register to add additional formats.
var Formats = []string{
	// ISO 8601 / RFC 3339 (most specific first)
	DateTimeMicro,
	DateTimeMilliT,
	RFC3339Nano,
	ISO8601,

	// Space-separated date time (most specific first)
	DateTimeMicroTZ,
	DateTimeMilliTZ,
	DateTimeTZ,
	DateTimeMST,
	DateTimeMicro2,
	DateTimeMilli,
	DateTime,

	// Common/human-readable
	CommonDateTime,
	CommonDateTime2,

	// Apache/CLF
	ApacheCLF,

	// ANSIC / Unix
	UnixDate,
	ANSIC,
	ANSICDate,

	// RFC variants
	RFC1123,
	RFC1123Z,
	RFC850,
	RFC822,
	RFC822Z,

	// Syslog (no year - matched last as fallback)
	Syslog,
	SyslogDate,
}

// stripBrackets removes surrounding brackets from the first element in a string.
// For example, "[timestamp] [level]" becomes "timestamp [level]".
func stripBrackets(s string) string {
	if len(s) > 0 && s[0] == '[' {
		if i := strings.IndexByte(s, ']'); i != -1 {
			s = s[1:i] + s[i+1:]
		}
	}
	return s
}

// Parse attempts to parse the timestamp string against each layout in Formats.
// Surrounding brackets are stripped before parsing.
// Returns the first successful parse or an error if none match.
func Parse(value string) (time.Time, error) {
	value = stripBrackets(strings.TrimSpace(value))
	for _, layout := range Formats {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("logtime: unable to parse %q", value)
}

// MustParse is like Parse but panics on failure.
func MustParse(value string) time.Time {
	t, err := Parse(value)
	if err != nil {
		panic(err)
	}
	return t
}

// Register adds one or more time.Parse layout strings to Formats.
func Register(layouts ...string) {
	Formats = append(layouts, Formats...)
}
