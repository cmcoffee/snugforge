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
	w_buff    *bytes.Buffer
	source    io.ReadCloser
	eof       bool
	f_writer  io.Writer
	mwrite    *multipart.Writer
}

// Close closes the underlying source. If a chunk size is defined,
// it does nothing.
func (s *streamReadCloser) Close() (err error) {
	if s.chunkSize > 0 {
		return nil
	} else {
		return s.source.Close()
	}
}

// Read reads from the stream.
func (s *streamReadCloser) Read(p []byte) (n int, err error) {

	// If we have stuff in our output buffer, read from there.
	// If not, reset the bytes buffer and read from source.
	if s.w_buff.Len() > 0 {
		return s.w_buff.Read(p)
	} else {
		s.w_buff.Reset()
	}

	// We've reached the EOF, return to process.
	if s.eof {
		return 0, io.EOF
	}

	// Get length of incoming []byte slice.
	p_len := int64(len(p))

	if sz := s.chunkSize - s.size; sz > 0 || sz == -1 {
		if sz > p_len || sz == -1 {
			sz = p_len
		}

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

	// Close out the mime stream.
	if s.eof {
		s.mwrite.Close()
	}

	return s.w_buff.Read(p)
}

// ConvertFormFile converts the request body to multipart/form-data.
// It allows adding extra fields and limits the byte size.
// The `fieldname` and `filename` are used for the form field/file.
func ConvertFormFile(request *http.Request, fieldname string, filename string, add_fields map[string]string, byte_limit int64) {
	convertBody(request, fieldname, filename, add_fields, byte_limit)
}

// ConvertForm converts the request body to multipart/form-data.
// It adds the given fields to the form data.
func ConvertForm(request *http.Request, fieldname string, add_fields map[string]string) {
	convertBody(request, fieldname, "", add_fields, -1)
}

// convertBody converts an HTTP request body to a multipart/form-data body.
// It adds the given fields and file to the request.
// byte_limit limits the total size of the request body.
func convertBody(request *http.Request, fieldname string, filename string, fields map[string]string, byte_limit int64) {
	if request == nil || request.Body == nil {
		return
	}

	w_buff := new(bytes.Buffer)
	w := multipart.NewWriter(w_buff)

	for k, v := range fields {
		w.WriteField(k, v)
	}

	var f_writer io.Writer

	if filename == "" {
		f_writer, _ = w.CreateFormField(fieldname)
	} else {
		f_writer, _ = w.CreateFormFile(fieldname, filename)
	}

	request.Header.Set("Content-Type", "multipart/form-data; boundary="+w.Boundary())

	request.Body = &streamReadCloser{
		byte_limit,
		0,
		w_buff,
		request.Body,
		false,
		f_writer,
		w,
	}
}
