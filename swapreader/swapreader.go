package swapreader

import (
	"io"
)

// Reader provides a way to read data from either a byte slice or an io.Reader.
type Reader struct {
	from_reader    bool
	reader         io.Reader
	decoder_bytes  []byte
	decoder_copied int
}

// SetBytes sets the underlying byte slice for reading.
// It disables reading from an io.Reader.
func (r *Reader) SetBytes(in []byte) {
	r.from_reader = false
	r.decoder_bytes = in
	r.decoder_copied = 0
}

// SetReader sets the underlying reader for decoding.
// It indicates that the Reader will receive input from an io.Reader.
func (r *Reader) SetReader(in io.Reader) {
	r.from_reader = true
	r.reader = in
}

// Read reads bytes from the internal buffer or reader.
// It returns the number of bytes read and a possible error.
func (r *Reader) Read(p []byte) (n int, err error) {

	if !r.from_reader {
		buffer_len := len(r.decoder_bytes) - r.decoder_copied

		if len(p) <= buffer_len {
			for i := 0; i < len(p); i++ {
				p[i] = r.decoder_bytes[r.decoder_copied]
				r.decoder_copied++
			}
		} else {
			for i := 0; i < buffer_len; i++ {
				p[i] = r.decoder_bytes[r.decoder_copied]
				r.decoder_copied++
			}
		}

		transferred := len(r.decoder_bytes) - r.decoder_copied

		if transferred == 0 {
			err = io.EOF
		}

		return buffer_len - transferred, err
	} else {
		return r.Read(p)
	}

}
