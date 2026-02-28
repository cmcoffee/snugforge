/*
	Package iotimeout provides a configurable timeout for io.Reader and io.ReadCloser.
*/

package iotimeout

import (
	"errors"
	. "github.com/cmcoffee/snugforge/xsync"
	"io"
	"sync"
	"time"
)

// ErrTimeout indicates that a timeout was reached while waiting.
var ErrTimeout = errors.New("Timeout reached while waiting for bytes.")

// waiting represents the initial state of a process.
// halted represents a process that has been stopped.
// timedout distinguishes a timeout halt from an explicit close.
const (
	waiting = 1 << iota
	halted
	timedout
)

// start_timer manages a timeout for processing input.
// It signals completion via a channel when the timeout is reached.
func start_timer(timeout time.Duration, flag *BitFlag, input chan []byte, expired chan struct{}) {
	if timeout <= 0 {
		for !flag.Has(halted) {
			time.Sleep(50 * time.Millisecond)
		}
		input <- nil
		return
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var waitStart time.Time

	for range ticker.C {
		if flag.Has(halted) {
			input <- nil
			return
		}
		if flag.Has(waiting) {
			if waitStart.IsZero() {
				waitStart = time.Now()
			} else if time.Since(waitStart) >= timeout {
				flag.Set(halted | timedout)
				expired <- struct{}{}
				input <- nil
				return
			}
		} else {
			waitStart = time.Time{}
			flag.Set(waiting)
		}
	}
}

// resp holds the number of bytes read and any error encountered.
// It's used to communicate read results from a goroutine.
type resp struct {
	n   int
	err error
}

// readCloser manages a timed read operation on an io.ReadCloser.
// It provides a mechanism to halt the read operation after a specified timeout.
type readCloser struct {
	src     io.ReadCloser
	flag    BitFlag
	input   chan []byte
	output  chan resp
	expired chan struct{}
	mutex   sync.Mutex
}

// reader wraps an io.Reader. It implements io.ReadCloser by always returning nil for Close.
type reader struct {
	io.Reader
}

// Close closes the reader.
func (r reader) Close() (err error) {
	return nil
}

// NewReader returns a new timed reader.
// It wraps the provided io.Reader with a timeout.
func NewReader(source io.Reader, timeout time.Duration) io.Reader {
	return NewReadCloser(reader{source}, timeout)
}

// NewReadCloser returns a new ReadCloser with a timeout.
// It wraps the given io.ReadCloser and adds a timeout mechanism.
func NewReadCloser(source io.ReadCloser, timeout time.Duration) io.ReadCloser {
	t := new(readCloser)
	if source == nil {
		return source
	}
	t.src = source
	t.input = make(chan []byte, 1)
	t.output = make(chan resp, 1)
	t.expired = make(chan struct{}, 1)

	go start_timer(timeout, &t.flag, t.input, t.expired)

	go func() {
		var (
			data resp
			p    []byte
		)
		for {
			p = <-t.input
			if p == nil {
				break
			}
			t.flag.Unset(waiting)
			data.n, data.err = source.Read(p)
			t.output <- data
		}
	}()
	return t
}

// Read reads up to len(p) bytes from the underlying reader.
func (t *readCloser) Read(p []byte) (n int, err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.flag.Has(halted) {
		if t.flag.Has(timedout) {
			return 0, ErrTimeout
		}
		return 0, io.ErrClosedPipe
	}

	t.input <- p

	select {
	case data := <-t.output:
		n = data.n
		err = data.err
		t.flag.Set(waiting)
		if err != nil {
			t.flag.Set(halted)
		}
	case <-t.expired:
		t.flag.Set(halted | timedout)
		t.src.Close() // interrupt the goroutine's blocking read
		<-t.output    // wait for goroutine to finish with p before returning
		return 0, ErrTimeout
	}
	return
}

// Close closes the underlying reader.
func (t *readCloser) Close() (err error) {
	t.flag.Set(halted)
	return t.src.Close()
}
