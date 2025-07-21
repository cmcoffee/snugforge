# swapreader
--
    import "github.com/cmcoffee/snugforge/swapreader"


## Usage

#### type Reader

```go
type Reader struct {
}
```

Reader provides a way to read data from either a byte slice or an io.Reader.

#### func (*Reader) Read

```go
func (r *Reader) Read(p []byte) (n int, err error)
```
Read reads bytes from the internal buffer or reader. It returns the number of
bytes read and a possible error.

#### func (*Reader) SetBytes

```go
func (r *Reader) SetBytes(in []byte)
```
SetBytes sets the underlying byte slice for reading. It disables reading from an
io.Reader.

#### func (*Reader) SetReader

```go
func (r *Reader) SetReader(in io.Reader)
```
SetReader sets the underlying reader for decoding. It indicates that the Reader
will receive input from an io.Reader.
