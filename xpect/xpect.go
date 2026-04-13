/*
Package xpect provides expect-like automation for interactive command-line programs.

It allows spawning a process, sending input, and waiting for expected output
patterns with configurable timeouts — similar to the Unix expect tool.
*/
package xpect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

// ErrTimeout indicates that an expect operation timed out waiting for a match.
var ErrTimeout = errors.New("Timeout reached while waiting for expected output.")

// ErrClosed indicates that the session has already been closed.
var ErrClosed = errors.New("Session is closed.")

// Default timeout for expect operations when none is specified.
const DefaultTimeout = 30 * time.Second

// Match holds the result of a successful expect operation.
type Match struct {
	// Before contains all output received before the match.
	Before string

	// Full contains the full text matched by the pattern.
	Full string

	// Groups contains any capture group matches from the pattern.
	Groups []string
}

// Session manages an interactive process and provides expect-style operations.
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

	cmd         *exec.Cmd
	stdin       io.WriteCloser
	buf         bytes.Buffer
	readErr     error
	readCh      chan struct{}
	timeout     time.Duration
	closed      bool
	interactOut io.Writer
	interactDn  chan struct{}
	mutex       sync.Mutex
}

// Command starts a new interactive session with the given command and arguments.
// The process is started immediately. Returns an error if the process fails to start.
func Command(name string, args ...string) (*Session, error) {
	cmd := exec.Command(name, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("xpect: failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("xpect: failed to create stdout pipe: %w", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("xpect: failed to start command: %w", err)
	}

	s := &Session{
		cmd:     cmd,
		stdin:   stdin,
		readCh:  make(chan struct{}, 1),
		timeout: DefaultTimeout,
	}

	go s.reader(stdout)

	return s, nil
}

// reader continuously reads from the process output into the buffer.
func (s *Session) reader(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.mutex.Lock()
			s.buf.Write(buf[:n])
			if s.Log != nil {
				s.Log.Write(buf[:n])
			}
			if s.interactOut != nil {
				s.interactOut.Write(buf[:n])
			}
			s.mutex.Unlock()

			select {
			case s.readCh <- struct{}{}:
			default:
			}
		}
		if err != nil {
			s.mutex.Lock()
			s.readErr = err
			s.mutex.Unlock()

			select {
			case s.readCh <- struct{}{}:
			default:
			}
			return
		}
	}
}

// SetTimeout sets the default timeout for expect operations.
func (s *Session) SetTimeout(d time.Duration) {
	s.timeout = d
}

// Expect waits for the process output to match the given regular expression pattern.
// It uses the session's default timeout.
func (s *Session) Expect(pattern string) (Match, error) {
	return s.ExpectTimeout(pattern, s.timeout)
}

// ExpectTimeout waits for the process output to match the given regular expression
// pattern within the specified timeout duration.
func (s *Session) ExpectTimeout(pattern string, timeout time.Duration) (Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Match{}, fmt.Errorf("xpect: invalid pattern: %w", err)
	}

	deadline := time.After(timeout)

	for {
		s.mutex.Lock()
		if s.closed {
			s.mutex.Unlock()
			return Match{}, ErrClosed
		}
		data := s.buf.String()
		s.mutex.Unlock()

		if loc := re.FindStringSubmatchIndex(data); loc != nil {
			m := Match{
				Before: data[:loc[0]],
				Full:   data[loc[0]:loc[1]],
			}
			for i := 2; i < len(loc); i += 2 {
				if loc[i] >= 0 {
					m.Groups = append(m.Groups, data[loc[i]:loc[i+1]])
				} else {
					m.Groups = append(m.Groups, "")
				}
			}

			// Consume matched output from buffer.
			s.mutex.Lock()
			s.buf.Reset()
			s.buf.WriteString(data[loc[1]:])
			s.mutex.Unlock()

			return m, nil
		}

		// Check if process output has ended with no match.
		s.mutex.Lock()
		readErr := s.readErr
		s.mutex.Unlock()

		if readErr != nil {
			return Match{}, fmt.Errorf("xpect: process output ended without match: %w", readErr)
		}

		select {
		case <-deadline:
			return Match{}, ErrTimeout
		case <-s.readCh:
		}
	}
}

// ExpectEOF waits for the process to close its output stream.
// It uses the session's default timeout.
func (s *Session) ExpectEOF() error {
	return s.ExpectEOFTimeout(s.timeout)
}

// ExpectEOFTimeout waits for the process to close its output stream
// within the specified timeout duration.
func (s *Session) ExpectEOFTimeout(timeout time.Duration) error {
	deadline := time.After(timeout)

	for {
		s.mutex.Lock()
		if s.closed {
			s.mutex.Unlock()
			return ErrClosed
		}
		readErr := s.readErr
		s.mutex.Unlock()

		if readErr != nil {
			return nil
		}

		select {
		case <-deadline:
			return ErrTimeout
		case <-s.readCh:
		}
	}
}

// Send writes the given string to the process's standard input.
func (s *Session) Send(input string) error {
	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return ErrClosed
	}
	s.mutex.Unlock()

	if s.SendLog && s.Log != nil {
		s.mutex.Lock()
		if s.SendMask != "" {
			io.WriteString(s.Log, s.SendMask)
			s.SendMask = ""
		} else {
			io.WriteString(s.Log, input)
		}
		s.mutex.Unlock()
	} else if s.SendMask != "" {
		s.mutex.Lock()
		s.SendMask = ""
		s.mutex.Unlock()
	}

	_, err := io.WriteString(s.stdin, input)
	if err != nil {
		return fmt.Errorf("xpect: failed to send input: %w", err)
	}
	return nil
}

// SendLine writes the given string followed by a newline to the process's standard input.
func (s *Session) SendLine(input string) error {
	return s.Send(input + "\n")
}

// Output returns any unmatched output currently in the buffer.
func (s *Session) Output() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.buf.String()
}

// Clear discards any unmatched output currently in the buffer.
func (s *Session) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.buf.Reset()
}

// Interact hands control to the user for live two-way interaction with the process.
// User input from r is forwarded to the process, and process output is written to w.
// Blocks until the process exits or the input reader returns an error (e.g. EOF).
// After Interact returns, scripted Expect/Send calls can resume.
func (s *Session) Interact(r io.Reader, w io.Writer) error {
	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return ErrClosed
	}
	s.interactOut = w
	s.interactDn = make(chan struct{})

	// Flush any buffered output to the user.
	if s.buf.Len() > 0 {
		w.Write(s.buf.Bytes())
	}
	s.mutex.Unlock()

	defer func() {
		s.mutex.Lock()
		s.interactOut = nil
		close(s.interactDn)
		s.interactDn = nil
		s.mutex.Unlock()
	}()

	// Forward user input to process stdin.
	go func() {
		io.Copy(s.stdin, r)
	}()

	// Wait for the process output to end.
	for {
		s.mutex.Lock()
		readErr := s.readErr
		s.mutex.Unlock()
		if readErr != nil {
			return nil
		}

		<-s.readCh
	}
}

// InteractUntil hands control to the user like Interact, but returns control to
// the script when the process output matches the given regular expression pattern.
// The matched output is consumed from the buffer, and the Match result is returned.
func (s *Session) InteractUntil(r io.Reader, w io.Writer, pattern string) (Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Match{}, fmt.Errorf("xpect: invalid pattern: %w", err)
	}

	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return Match{}, ErrClosed
	}
	s.interactOut = w
	s.interactDn = make(chan struct{})

	// Flush any buffered output to the user.
	if s.buf.Len() > 0 {
		w.Write(s.buf.Bytes())
	}
	s.mutex.Unlock()

	defer func() {
		s.mutex.Lock()
		s.interactOut = nil
		close(s.interactDn)
		s.interactDn = nil
		s.mutex.Unlock()
	}()

	// Forward user input to process stdin in the background.
	// Use a pipe so we can stop forwarding when the pattern matches.
	pr, pw := io.Pipe()
	go func() {
		io.Copy(pw, r)
		pw.Close()
	}()
	go func() {
		io.Copy(s.stdin, pr)
	}()
	defer pr.Close()

	for {
		s.mutex.Lock()
		data := s.buf.String()
		s.mutex.Unlock()

		if loc := re.FindStringSubmatchIndex(data); loc != nil {
			m := Match{
				Before: data[:loc[0]],
				Full:   data[loc[0]:loc[1]],
			}
			for i := 2; i < len(loc); i += 2 {
				if loc[i] >= 0 {
					m.Groups = append(m.Groups, data[loc[i]:loc[i+1]])
				} else {
					m.Groups = append(m.Groups, "")
				}
			}

			s.mutex.Lock()
			s.buf.Reset()
			s.buf.WriteString(data[loc[1]:])
			s.mutex.Unlock()

			return m, nil
		}

		s.mutex.Lock()
		readErr := s.readErr
		s.mutex.Unlock()

		if readErr != nil {
			return Match{}, fmt.Errorf("xpect: process output ended without match: %w", readErr)
		}

		<-s.readCh
	}
}

// Wait waits for the process to exit and returns its exit status.
func (s *Session) Wait() error {
	return s.cmd.Wait()
}

// Close closes the session by closing stdin and killing the process.
func (s *Session) Close() error {
	s.mutex.Lock()
	if s.closed {
		s.mutex.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.mutex.Unlock()

	s.stdin.Close()

	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	return s.cmd.Wait()
}
