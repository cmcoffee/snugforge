package swapreader

import (
	"io"
)

// Reader provides a way to read data from either a byte slice or an io.Reader.
type Reader struct {
	fromReader   bool
	reader       io.Reader
	decoderBytes []byte
	bytesCopied  int
}

// SetBytes sets the underlying byte slice for reading.
// It disables reading from an io.Reader.
func (r *Reader) SetBytes(in []byte) {
	r.fromReader = false
	r.decoderBytes = in
	r.bytesCopied = 0
}

// SetReader sets the underlying reader for decoding.
// It indicates that the Reader will receive input from an io.Reader.
func (r *Reader) SetReader(in io.Reader) {
	r.fromReader = true
	r.reader = in
}

// Read reads bytes from the internal buffer or reader.
// It returns the number of bytes read and a possible error.
func (r *Reader) Read(p []byte) (n int, err error) {
	if !r.fromReader {
		n = copy(p, r.decoderBytes[r.bytesCopied:])
		r.bytesCopied += n
		if r.bytesCopied >= len(r.decoderBytes) {
			err = io.EOF
		}
		return n, err
	}
	return r.reader.Read(p)
}
