# csvp
--
    import "github.com/cmcoffee/snugforge/csvp"


## Usage

#### func  IsReadError

```go
func IsReadError(err error) bool
```
IsReadError reports whether err is a row read error. It checks if the error is
of type rowReadError.

#### func  IsRowError

```go
func IsRowError(err error) bool
```
IsRowError checks if an error is a rowProcessError. It returns true if the error
is of type rowProcessError, otherwise it returns false.

#### type CSVReader

```go
type CSVReader struct {
	Processor    func(row []string) (err error)                     // Callback funcction for each row read.
	ErrorHandler func(line int, row string, err error) (abort bool) // ErrorHandler when problem reading CSV or processing CSV.
}
```

CSVReader processes CSV data row by row.

#### func  NewReader

```go
func NewReader() *CSVReader
```
NewReader creates and returns a new CSVReader instance.

#### func (*CSVReader) Read

```go
func (T *CSVReader) Read(reader io.Reader)
```
Read reads a CSV from the provided reader, processing each row. It skips lines
starting with '#'.
