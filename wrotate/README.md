# wrotate
--
    import "github.com/cmcoffee/snugforge/wrotate"


## Usage

#### func  OpenFile

```go
func OpenFile(name string, max_bytes int64, max_rotations uint) (io.WriteCloser, error)
```
OpenFile opens or creates a file, optionally rotating it based on size and
rotations. It returns a WriteCloser and an error if file opening fails.
