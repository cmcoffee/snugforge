/*
Package logtail provides continuous log file tailing with pattern-matched callbacks.

It watches a log file for new content, parses timestamps using the logtime package,
and fires registered callbacks when lines match specified regular expression patterns.
Timestamp tracking provides a safety net against reprocessing entries after file
rotation or tailer restart.
*/
package logtail

import (
	"github.com/cmcoffee/snugforge/logtime"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Default poll interval for checking new file content.
const DefaultInterval = 250 * time.Millisecond

// Match holds the result of a pattern match against a log entry.
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

// rule binds a compiled regex to a callback function.
type rule struct {
	re *regexp.Regexp
	fn func(Match)
}

// Tailer watches a log file and fires callbacks when patterns match new entries.
type Tailer struct {
	path       string
	interval   time.Duration
	from_start bool
	rules      []rule

	// timestamp parsing state
	layout    string
	tsLen     int
	last_seen time.Time

	// shutdown
	done chan struct{}
	once sync.Once
}

// Open creates a new Tailer for the given log file path.
// The file is not opened until Run is called.
func Open(path string) *Tailer {
	return &Tailer{
		path:     path,
		interval: DefaultInterval,
		done:     make(chan struct{}),
	}
}

// SetInterval sets the poll interval for checking new file content.
func (t *Tailer) SetInterval(d time.Duration) {
	t.interval = d
}

// FromStart configures the tailer to process existing file content
// instead of seeking to the end on start.
func (t *Tailer) FromStart() {
	t.from_start = true
}

// On registers a callback that fires when a new log entry matches the
// given regular expression pattern. The pattern is matched against the
// timestamp-stripped text of each entry. Multiple rules may be registered
// and all matching rules fire for each entry.
func (t *Tailer) On(pattern string, fn func(Match)) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	t.rules = append(t.rules, rule{re, fn})
	return nil
}

// MustOn is like On but panics if the pattern fails to compile.
func (t *Tailer) MustOn(pattern string, fn func(Match)) {
	if err := t.On(pattern, fn); err != nil {
		panic(err)
	}
}

// Run begins tailing the log file. It blocks until Close is called or
// an unrecoverable error occurs. File rotation and truncation are handled
// automatically by detecting size changes and re-opening the file.
func (t *Tailer) Run() error {
	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Seek to end unless FromStart was called.
	if !t.from_start {
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return err
		}
	}

	var (
		remainder string
		last_size int64
		ticker    = time.NewTicker(t.interval)
	)
	defer ticker.Stop()

	// Get initial file size.
	if info, err := f.Stat(); err == nil {
		last_size = info.Size()
	}

	for {
		select {
		case <-t.done:
			return nil
		case <-ticker.C:
		}

		// Check for file rotation or truncation.
		f, last_size, err = t.check_rotation(f, last_size)
		if err != nil {
			return err
		}

		// Read any new content.
		buf := make([]byte, 4096)
		for {
			n, read_err := f.Read(buf)
			if n > 0 {
				remainder = t.process(remainder + string(buf[:n]))
			}
			if read_err != nil {
				break
			}
		}

		// Update tracked size.
		if info, err := f.Stat(); err == nil {
			last_size = info.Size()
		}
	}
}

// Close stops the tailer. It is safe to call from any goroutine.
func (t *Tailer) Close() {
	t.once.Do(func() { close(t.done) })
}

// check_rotation detects file truncation or replacement and re-opens as needed.
func (t *Tailer) check_rotation(f *os.File, last_size int64) (*os.File, int64, error) {
	info, err := f.Stat()
	if err != nil {
		// File may have been removed; try to re-open.
		f.Close()
		return t.reopen()
	}

	// Check if the file was replaced (different inode) by stat'ing the path.
	path_info, path_err := os.Stat(t.path)
	if path_err != nil {
		return f, last_size, nil
	}
	if !os.SameFile(info, path_info) {
		// File was replaced (e.g., log rotation with new file).
		f.Close()
		return t.reopen()
	}

	// File was truncated (e.g., copytruncate rotation).
	if info.Size() < last_size {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return t.reopen()
		}
		return f, 0, nil
	}

	return f, last_size, nil
}

// reopen attempts to open the file, retrying on each poll tick until
// the file appears or Close is called.
func (t *Tailer) reopen() (*os.File, int64, error) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return nil, 0, nil
		case <-ticker.C:
		}

		f, err := os.Open(t.path)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		return f, info.Size(), nil
	}
}

// process splits raw data into lines, detects timestamps, groups continuation
// lines, and fires matching rules. Returns any incomplete trailing line.
func (t *Tailer) process(data string) string {
	lines := strings.Split(data, "\n")

	// Last element is either empty (line ended with \n) or an incomplete line.
	remainder := lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}
		t.handle_line(line)
	}

	return remainder
}

// handle_line parses a single log line and fires any matching rules.
func (t *Tailer) handle_line(line string) {
	ts, text := t.parse_timestamp(line)

	// Skip entries at or before the last seen timestamp (safety net for rotation).
	if !ts.IsZero() && !t.last_seen.IsZero() && !ts.After(t.last_seen) {
		return
	}
	if !ts.IsZero() {
		t.last_seen = ts
	}

	for _, r := range t.rules {
		loc := r.re.FindStringSubmatchIndex(text)
		if loc == nil {
			continue
		}

		m := Match{
			Time: ts,
			Text: text,
			Full: line,
		}

		for i := 2; i < len(loc); i += 2 {
			if loc[i] >= 0 {
				m.Groups = append(m.Groups, text[loc[i]:loc[i+1]])
			} else {
				m.Groups = append(m.Groups, "")
			}
		}

		r.fn(m)
	}
}

// parse_timestamp attempts to detect and parse a timestamp from the line.
// On first successful detection it locks in the layout for subsequent lines.
func (t *Tailer) parse_timestamp(line string) (time.Time, string) {
	// Layout already detected — use it.
	if t.layout != "" {
		if len(line) >= t.tsLen {
			raw := line[:t.tsLen]
			cleaned := strip_brackets(raw)
			if ts, err := time.Parse(t.layout, cleaned); err == nil {
				text := strings.TrimLeft(line[t.tsLen:], " ")
				return ts, text
			}
		}
		// Line doesn't match known layout; treat as continuation/no-timestamp.
		return time.Time{}, line
	}

	// Try to auto-detect from this line.
	layout, tsLen, err := detect_layout(line)
	if err != nil {
		return time.Time{}, line
	}
	t.layout = layout
	t.tsLen = tsLen

	cleaned := strip_brackets(line[:tsLen])
	ts, _ := time.Parse(layout, cleaned)
	text := strings.TrimLeft(line[tsLen:], " ")
	return ts, text
}

// detect_layout splits the line into space-delimited tokens and tries
// progressively longer prefixes against each logtime format.
func detect_layout(line string) (string, int, error) {
	fields := strings.Fields(line)
	for n := 1; n <= len(fields); n++ {
		candidate := strings.Join(fields[:n], " ")
		cleaned := strip_brackets(candidate)
		for _, layout := range logtime.Formats {
			if _, err := time.Parse(layout, cleaned); err == nil {
				return layout, len(candidate), nil
			}
		}
	}
	return "", 0, Error("logtail: unable to detect timestamp layout")
}

// strip_brackets removes surrounding brackets from a timestamp prefix.
func strip_brackets(s string) string {
	if len(s) > 0 && s[0] == '[' {
		if i := strings.IndexByte(s, ']'); i != -1 {
			s = s[1:i] + s[i+1:]
		}
	}
	return s
}

// Error is a simple string error type.
type Error string

func (e Error) Error() string { return string(e) }
