# mimebody
--
    import "github.com/cmcoffee/snugforge/mimebody"


## Usage

#### func  ConvertForm

```go
func ConvertForm(request *http.Request, fieldname string, add_fields map[string]string) error
```
ConvertForm converts the request body to multipart/form-data. It adds the given
fields to the form data.

#### func  ConvertFormFile

```go
func ConvertFormFile(request *http.Request, fieldname string, filename string, add_fields map[string]string, byte_limit int64) error
```
ConvertFormFile converts the request body to multipart/form-data. It allows
adding extra fields and limits the byte size. The `fieldname` and `filename` are
used for the form field/file.
