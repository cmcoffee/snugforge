# mimebody
--
    import "github.com/cmcoffee/snugforge/mimebody"


## Usage

#### func  ConvertForm

```go
func ConvertForm(request *http.Request, fieldname string, add_fields map[string]string)
```
Transforms body of request to mime multipart upload. Request body should be
io.ReadCloser of file being transfered. fieldname specifies field for content.

#### func  ConvertFormFile

```go
func ConvertFormFile(request *http.Request, fieldname string, filename string, add_fields map[string]string, byte_limit int64)
```
Transforms body of request to mime multipart upload. Request body should be
io.ReadCloser of file being transfered. fieldname specified field for content,
filename should be filename of file.
