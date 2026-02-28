package mimebody

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
)

// streamReadCloser manages reading from an io.ReadCloser with chunking.
// It buffers the read data and provides a way to limit the size of the read.
// It also handles multipart form data writing.
type streamReadCloser struct {
	chunkSize int64
	size      int64
	buffer    *bytes.Buffer
	source    io.ReadCloser
	eof       bool
	f_writer  io.Writer
	mwrite    *multipart.Writer
}

// Close closes the underlying source. If a chunk size is defined,
// it does nothing as the source must remain open for subsequent chunks.
func (s *streamReadCloser) Close() error {
	if s.chunkSize > 0 {
		return nil
	}
	if s.source != nil {
		return s.source.Close()
	}
	return nil
}

// Read reads from the stream.
func (s *streamReadCloser) Read(p []byte) (n int, err error) {

	// If we have stuff in our output buffer, read from there.
	// If not, reset the bytes buffer and read from source.
	if s.buffer.Len() > 0 {
		return s.buffer.Read(p)
	}

	s.buffer.Reset() // Resets buffer

	// We've reached the EOF, return to process.
	if s.eof {
		return 0, io.EOF
	}

	// Get length of incoming []byte slice.
	p_len := int64(len(p))

	var sz int64

	if s.chunkSize > 0 {
		remaining := s.chunkSize - s.size
		if remaining <= 0 {
			s.eof = true
		} else {
			if remaining < p_len {
				sz = remaining
			} else {
				sz = p_len
			}
		}
	} else {
		sz = p_len
	}

	if !s.eof && sz > 0 {
		// Read into the byte slice provided from source.
		n, err := s.source.Read(p[0:sz])
		if err != nil {
			if err == io.EOF {
				s.eof = true
			} else {
				return n, err
			}
		}

		// We're writing to a bytes.Buffer.
		_, err = s.f_writer.Write(p[0:n])
		if err != nil {
			return n, err
		}

		// Clear out the []byte slice provided.
		for i := 0; i < n; i++ {
			p[i] = 0
		}

		s.size = s.size + int64(n)
	} else {
		s.eof = true
	}

	// finalize multipart writer (writes closing boundary)
	if s.eof {
		if cerr := s.mwrite.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	// read newly written multipart data into p
	return s.buffer.Read(p)
}

// ConvertFormFile converts the request body to multipart/form-data.
// It allows adding extra fields and limits the byte size.
// The `fieldname` and `filename` are used for the form field/file.
func ConvertFormFile(request *http.Request, fieldname string, filename string, add_fields map[string]string, byte_limit int64) error {
	return convertBody(request, fieldname, filename, add_fields, byte_limit)
}

// ConvertForm converts the request body to multipart/form-data.
// It adds the given fields to the form data.
func ConvertForm(request *http.Request, fieldname string, add_fields map[string]string) error {
	return convertBody(request, fieldname, "", add_fields, -1)
}

// convertBody converts an HTTP request body to a multipart/form-data body.
// It adds the given fields and file to the request.
// byte_limit limits the source bytes read; 0 or negative means unlimited.
func convertBody(request *http.Request, fieldname string, filename string, fields map[string]string, byte_limit int64) error {
	if request == nil || request.Body == nil {
		return nil
	}

	buffer := new(bytes.Buffer)
	w := multipart.NewWriter(buffer)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return err
		}
	}

	var (
		f_writer io.Writer
		err      error
	)

	if filename == "" {
		f_writer, err = w.CreateFormField(fieldname)
	} else {
		f_writer, err = w.CreateFormFile(fieldname, filename)
	}
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "multipart/form-data; boundary="+w.Boundary())
	request.ContentLength = -1

	request.Body = &streamReadCloser{
		chunkSize: byte_limit,
		buffer:    buffer,
		source:    request.Body,
		f_writer:  f_writer,
		mwrite:    w,
	}
	return nil
}
