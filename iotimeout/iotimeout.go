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
const (
	waiting = 1 << iota
	halted
)

// start_timer manages a timeout for processing input.
// It signals completion via a channel when the timeout is reached.
func start_timer(timeout time.Duration, flag *BitFlag, input chan []byte, expired chan struct{}) {
	timeout_seconds := int64(timeout.Round(time.Second).Seconds())

	var cnt int64

	for {
		time.Sleep(time.Second)
		if flag.Has(halted) {
			input <- nil
			break
		}

		if flag.Has(waiting) {
			cnt++
			if timeout_seconds > 0 && cnt >= timeout_seconds {
				flag.Set(halted)
				expired <- struct{}{}
				input <- nil
				break
			}
		} else {
			cnt = 0
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
	t.input = make(chan []byte, 2)
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
		return t.src.Read(p)
	}

	// Set an idle timer.
	defer t.flag.Set(waiting)

	t.input <- p

	select {
	case data := <-t.output:
		n = data.n
		err = data.err
	case <-t.expired:
		t.flag.Set(halted)
		return -1, ErrTimeout
	}
	if err != nil {
		t.flag.Set(halted)
	}
	// Set an idle timer.
	return
}

// Close closes the underlying reader.
func (t *readCloser) Close() (err error) {
	t.flag.Set(halted)
	return t.src.Close()
}
