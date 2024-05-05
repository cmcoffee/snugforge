# csvp
--
    import "github.com/cmcoffee/snugforge/csvp"


## Usage

#### func  IsReadError

```go
func IsReadError(err error) bool
```
Returns true if error is generatored from reading the CSV.

#### func  IsRowError

```go
func IsRowError(err error) bool
```
Returns true if error is generated from processing the row of the CSV.

#### type CSVReader

```go
type CSVReader struct {
	Processor    func(row []string) (err error)                     // Callback funcction for each row read.
	ErrorHandler func(line int, row string, err error) (abort bool) // ErrorHandler when problem reading CSV or processing CSV.
}
```


#### func  NewReader

```go
func NewReader() *CSVReader
```
Allocates a New CSVReader.

#### func (*CSVReader) Read

```go
func (T *CSVReader) Read(reader io.Reader)
```
Reads incoming CSV data.
