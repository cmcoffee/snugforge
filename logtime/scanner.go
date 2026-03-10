package logtime

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

const defaultBufSize = 4096

// Scanner reads log entries from an io.Reader, grouping continuation lines
// (lines without a timestamp) with the preceding timestamped line, and
// filtering to entries whose timestamp falls within a start/stop window.
type Scanner struct {
	reader *bufio.Reader
	layout string
	tsLen   int
	start   time.Time
	stop    time.Time

	// buffered next entry (read-ahead from previous Scan)
	nextTime time.Time
	nextText string
	hasNext  bool

	// current entry returned by Text/Time
	text string
	ts   time.Time
	err  error
	done bool
}

// NewScanner creates a Scanner that reads log entries from r.
// The timestamp layout and prefix length are auto-detected from the first
// timestamped line in the reader. Lines before the first timestamp are skipped.
// Only entries with timestamps in [start, stop] are returned.
func NewScanner(start, stop time.Time, r io.Reader) (*Scanner, error) {
	s := &Scanner{
		reader: bufio.NewReaderSize(r, defaultBufSize),
		start:  start,
		stop:   stop,
	}

	// Read lines until we find one with a detectable timestamp.
	for {
		line, err := s.readLine()
		if err != nil {
			return nil, fmt.Errorf("logtime: no timestamped line found in input")
		}
		layout, tsLen, lerr := detectLayout(line)
		if lerr != nil {
			continue // skip non-timestamp lines
		}
		s.layout = layout
		s.tsLen = tsLen
		// Parse and buffer this first timestamped line.
		ts, _ := s.parseLine(line)
		s.nextTime = ts
		s.nextText = line
		s.hasNext = true
		return s, nil
	}
}

// detectLayout splits the example into space-delimited tokens and tries
// progressively longer prefixes against each registered format. This allows
// the example to be a full log line rather than just the timestamp portion.
// Returns the matching layout and the matched substring length.
func detectLayout(example string) (string, int, error) {
	fields := strings.Fields(example)
	for n := 1; n <= len(fields); n++ {
		candidate := strings.Join(fields[:n], " ")
		cleaned := stripBrackets(candidate)
		for _, layout := range Formats {
			if _, err := time.Parse(layout, cleaned); err == nil {
				return layout, len(candidate), nil
			}
		}
	}
	return "", 0, fmt.Errorf("logtime: unable to detect layout from example %q", example)
}

// Scan advances to the next log entry within the time window.
// It returns false when there are no more entries or an error occurs.
func (s *Scanner) Scan() bool {
	for {
		ts, text, ok := s.readEntry()
		if !ok {
			return false
		}

		// Past the stop time, we're done.
		if ts.After(s.stop) {
			s.done = true
			return false
		}

		// Before the start time, skip.
		if ts.Before(s.start) {
			continue
		}

		s.ts = ts
		s.text = text
		return true
	}
}

// readLine reads the next line from the reader, stripping the trailing newline.
// Returns the line, and an error (nil or io.EOF). On io.EOF the line may still
// contain the final unterminated line of input.
func (s *Scanner) readLine() (string, error) {
	line, err := s.reader.ReadString('\n')
	// Strip trailing newline/carriage-return.
	line = strings.TrimRight(line, "\r\n")
	if err == io.EOF && len(line) > 0 {
		// Final line without a trailing newline; return it without error
		// so the caller processes it, then the next call will return ""/EOF.
		return line, nil
	}
	return line, err
}

// readEntry reads the next complete log entry (timestamp line + any
// continuation lines). Returns the timestamp, full text, and whether
// an entry was read.
func (s *Scanner) readEntry() (time.Time, string, bool) {
	if s.done {
		return time.Time{}, "", false
	}

	// Use the buffered read-ahead if we have one.
	var entryTime time.Time
	var builder strings.Builder

	if s.hasNext {
		entryTime = s.nextTime
		builder.WriteString(s.nextText)
		s.hasNext = false
	} else {
		// Find the first timestamped line.
		for {
			line, err := s.readLine()
			if err != nil {
				s.err = errOrNil(err)
				return time.Time{}, "", false
			}
			ts, ok := s.parseLine(line)
			if ok {
				entryTime = ts
				builder.WriteString(line)
				break
			}
			// Lines before the first timestamp are discarded.
		}
	}

	// Collect continuation lines until we hit the next timestamp or EOF.
	for {
		line, err := s.readLine()
		if err != nil {
			s.err = errOrNil(err)
			s.done = true
			return entryTime, builder.String(), true
		}
		ts, ok := s.parseLine(line)
		if ok {
			// This line starts a new entry; buffer it for next call.
			s.nextTime = ts
			s.nextText = line
			s.hasNext = true
			return entryTime, builder.String(), true
		}
		builder.WriteByte('\n')
		builder.WriteString(line)
	}
}

// errOrNil converts io.EOF to nil, passing through any real errors.
func errOrNil(err error) error {
	if err == io.EOF {
		return nil
	}
	return err
}

// parseLine attempts to extract a timestamp from the beginning of a line.
func (s *Scanner) parseLine(line string) (time.Time, bool) {
	if len(line) < s.tsLen {
		return time.Time{}, false
	}
	t, err := time.Parse(s.layout, stripBrackets(line[:s.tsLen]))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Text returns the full text of the current log entry, including continuation lines.
func (s *Scanner) Text() string {
	return s.text
}

// Time returns the parsed timestamp of the current log entry.
func (s *Scanner) Time() time.Time {
	return s.ts
}

// Err returns the first non-EOF error encountered by the Scanner.
func (s *Scanner) Err() error {
	return s.err
}
