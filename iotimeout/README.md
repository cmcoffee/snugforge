# iotimeout
--
    import "github.com/cmcoffee/snugforge/iotimeout"


## Usage

```go
var ErrTimeout = errors.New("Timeout reached while waiting for bytes.")
```
ErrTimeout indicates that a timeout was reached while waiting.

#### func  NewReadCloser

```go
func NewReadCloser(source io.ReadCloser, timeout time.Duration) io.ReadCloser
```
NewReadCloser returns a new ReadCloser with a timeout. It wraps the given
io.ReadCloser and adds a timeout mechanism.

#### func  NewReader

```go
func NewReader(source io.Reader, timeout time.Duration) io.Reader
```
NewReader returns a new timed reader. It wraps the provided io.Reader with a
timeout.
