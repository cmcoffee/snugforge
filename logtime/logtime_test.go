package logtime

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	utc := time.FixedZone("", 0)

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		// Original four examples
		{"DateTimeMilli", "2026-03-05 03:28:19.308", time.Date(2026, 3, 5, 3, 28, 19, 308000000, time.UTC)},
		{"ISO8601", "2026-03-05T09:01:20+00:00", time.Date(2026, 3, 5, 9, 1, 20, 0, utc)},
		{"BracketCommon", "[23-Dec-2025 19:57:43 UTC]", time.Date(2025, 12, 23, 19, 57, 43, 0, time.UTC)},
		{"BracketMicro", "[2026-02-05T00:46:12.878593+00:00]", time.Date(2026, 2, 5, 0, 46, 12, 878593000, utc)},

		// ISO 8601 variants
		{"DateTimeMicro", "2026-03-05T09:01:20.123456+00:00", time.Date(2026, 3, 5, 9, 1, 20, 123456000, utc)},
		{"DateTimeMilliT", "2026-03-05T09:01:20.123+00:00", time.Date(2026, 3, 5, 9, 1, 20, 123000000, utc)},
		{"RFC3339Nano", "2026-03-05T09:01:20.123456789+00:00", time.Date(2026, 3, 5, 9, 1, 20, 123456789, utc)},

		// Space-separated variants
		{"DateTimeTZ", "2026-03-05 03:28:19 +00:00", time.Date(2026, 3, 5, 3, 28, 19, 0, utc)},
		{"DateTimeMST", "2026-03-05 03:28:19 UTC", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},
		{"DateTime", "2026-03-05 03:28:19", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},

		// Common/human-readable
		{"CommonSlash", "05/Mar/2026 03:28:19 UTC", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},

		// Apache CLF
		{"ApacheCLF", "05/Mar/2026:03:28:19 +0000", time.Date(2026, 3, 5, 3, 28, 19, 0, utc)},

		// ANSIC / Unix
		{"ANSIC", "Thu Mar  5 03:28:19 2026", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},
		{"UnixDate", "Thu Mar  5 03:28:19 UTC 2026", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},

		// RFC variants
		{"RFC1123", "Thu, 05 Mar 2026 03:28:19 UTC", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},
		{"RFC1123Z", "Thu, 05 Mar 2026 03:28:19 +0000", time.Date(2026, 3, 5, 3, 28, 19, 0, utc)},
		{"RFC850", "Thursday, 05-Mar-26 03:28:19 UTC", time.Date(2026, 3, 5, 3, 28, 19, 0, time.UTC)},
		{"RFC822", "05 Mar 26 03:28 UTC", time.Date(2026, 3, 5, 3, 28, 0, 0, time.UTC)},

		// Syslog (no year - defaults to year 0)
		{"Syslog", "Mar  5 03:28:19", time.Date(0, 3, 5, 3, 28, 19, 0, time.UTC)},
		{"SyslogDate", "Mar 5 03:28:19", time.Date(0, 3, 5, 3, 28, 19, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse("not-a-timestamp")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestRegister(t *testing.T) {
	custom := "Jan 2 2006 15:04:05"
	Register(custom)
	defer func() {
		// Clean up: remove the registered format.
		Formats = Formats[1:]
	}()

	got, err := Parse("Mar 5 2026 12:00:00")
	if err != nil {
		t.Fatalf("Parse with custom format error: %v", err)
	}
	want := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
