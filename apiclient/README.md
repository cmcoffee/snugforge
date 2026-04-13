# apiclient
--
    import "github.com/cmcoffee/snugforge/apiclient"

Package apiclient provides a reusable HTTP API client with OAuth2
authentication, automatic retries, rate limiting, pagination, and pluggable
error scanning.

It handles token lifecycle management (acquisition, refresh, storage), request
building with JSON/form/multipart payloads, and exponential backoff on transient
failures. Token storage is pluggable via the TokenStore interface, with a default
implementation backed by kvlite.

## Usage

#### type APIClient

```go
type APIClient struct {
	Server         string                               // API host name.
	ApplicationID  string                               // OAuth2 client ID.
	RedirectURI    string                               // OAuth2 redirect URI.
	RefreshPath    string                               // OAuth2 token refresh endpoint path (default "/oauth/token").
	AgentString    string                               // User-Agent header value.
	VerifySSL      bool                                 // Verify TLS certificates.
	ProxyURI       string                               // HTTPS proxy URL.
	RequestTimeout time.Duration                        // Timeout for API responses.
	ConnectTimeout time.Duration                        // Timeout for TLS handshake.
	MaxChunkSize   int64                                // Max upload chunk size in bytes.
	Flags          xsync.BitFlag                        // General-purpose flags.
	Retries        uint                                 // Max retries on failure.
	StaticToken    string                               // Static Bearer token; bypasses OAuth2 when set.
	TokenStore     TokenStore                           // Pluggable token storage.
	ReacquireToken bool                                 // Reacquire token on expiry instead of failing.
	URLScheme      string                               // URL scheme for requests ("http" or "https"). Defaults to "https".
	AuthFunc       func(req *http.Request)              // Custom auth function called per-request. Overrides Bearer token logic when set.
	Config         apiConfig                            // Encrypted in-memory config (e.g., client secret).
	NewToken       func(username string) (*Auth, error)  // Callback to acquire a new access token.
	ErrorScanner   func(body []byte) APIError            // Custom response error parser.
	RetryErrorCodes []string                             // Error codes that trigger a retry.
	TokenErrorCodes []string                             // Error codes indicating an invalid token.
}
```

APIClient is an HTTP API client with OAuth2 authentication, retries, and rate
limiting.

#### func (*APIClient) BackoffTimer

```go
func (s *APIClient) BackoffTimer(retry uint)
```
BackoffTimer pauses with exponential backoff: (retry+1)^2 seconds.

#### func (*APIClient) Call

```go
func (s *APIClient) Call(api_req APIRequest) (err error)
```
Call performs a structured API request with rate limiting and retry logic.

#### func (*APIClient) ClientSecret

```go
func (s *APIClient) ClientSecret(client_secret_key string)
```
ClientSecret stores the client secret (encrypted in memory).

#### func (*APIClient) Fulfill

```go
func (s *APIClient) Fulfill(username string, req *http.Request, output interface{}) (err error)
```
Fulfill executes an HTTP request with retry logic and decodes the JSON response
into output.

#### func (*APIClient) GetClientSecret

```go
func (s *APIClient) GetClientSecret() string
```
GetClientSecret retrieves the decrypted client secret.

#### func (*APIClient) GetLimit

```go
func (s *APIClient) GetLimit() int
```
GetLimit returns the current API rate limit capacity.

#### func (*APIClient) GetTransferLimit

```go
func (s *APIClient) GetTransferLimit() int
```
GetTransferLimit returns the transfer limiter capacity.

#### func (*APIClient) InitRetry

```go
func (s *APIClient) InitRetry(username string, task_description string, addtl_retry_error_codes ...string) *APIRetryEngine
```
InitRetry creates a new APIRetryEngine for the given user and task. Additional
error codes can trigger retries beyond those in RetryErrorCodes.

#### func (*APIClient) NewRequest

```go
func (s *APIClient) NewRequest(method, path string) (req *http.Request, err error)
```
NewRequest creates an HTTP(S) request to the configured server.

#### func (*APIClient) NewRequestWithContext

```go
func (s *APIClient) NewRequestWithContext(ctx context.Context, method, path string) (req *http.Request, err error)
```
NewRequestWithContext creates an HTTP(S) request with a context for cancellation.

#### func (*APIClient) PageCall

```go
func (s *APIClient) PageCall(req APIRequest, offset, limit int) (err error)
```
PageCall paginates through API responses, accumulating results from successive
calls with increasing offsets until all data is retrieved.

#### func (*APIClient) SendRequest

```go
func (s *APIClient) SendRequest(username string, req *http.Request) (resp *http.Response, err error)
```
SendRequest sends an HTTP request, setting the auth token if a username is
provided. The response body is consumed for error checking; use SendRawRequest
for streaming.

#### func (*APIClient) SendRawRequest

```go
func (s *APIClient) SendRawRequest(username string, req *http.Request) (resp *http.Response, err error)
```
SendRawRequest sends an HTTP request and returns the raw response without
reading or error-checking the body. The caller is responsible for closing
resp.Body. This is intended for streaming responses (SSE, chunked JSON) where
the body must be read incrementally.

#### func (*APIClient) SetDatabase

```go
func (s *APIClient) SetDatabase(db kvlite.Store)
```
SetDatabase sets the kvlite store and initializes the default TokenStore.

#### func (*APIClient) SetLimiter

```go
func (s *APIClient) SetLimiter(max_calls int)
```
SetLimiter configures the rate limiter for API calls.

#### func (*APIClient) SetToken

```go
func (s *APIClient) SetToken(username string, req *http.Request) (err error)
```
SetToken sets the authorization header for the given user. If AuthFunc is set, it
is called directly. If StaticToken is set, it is used as a Bearer token.
Otherwise, the token is loaded from the TokenStore, refreshed if expired, or
acquired via the NewToken callback.

#### func (*APIClient) SetTransferLimiter

```go
func (s *APIClient) SetTransferLimiter(max_transfers int)
```
SetTransferLimiter configures the concurrent transfer limiter.

#### type APIError

```go
type APIError struct {
}
```

APIError represents one or more errors returned by an API response.

#### func (APIError) Error

```go
func (e APIError) Error() string
```
Error returns the formatted error string.

#### func (*APIError) Register

```go
func (e *APIError) Register(code, message string)
```
Register adds an error code and message to the APIError.

#### type APIRequest

```go
type APIRequest struct {
	Username string
	Version  int
	Header   http.Header
	Method   string
	Path     string
	Params   []interface{}
	Output   interface{}
}
```

APIRequest represents a structured API call.

#### type APIRetryEngine

```go
type APIRetryEngine struct {
}
```

APIRetryEngine manages retry logic for API calls with exponential backoff.

#### func (*APIRetryEngine) CheckForRetry

```go
func (a *APIRetryEngine) CheckForRetry(err error) bool
```
CheckForRetry determines whether a retry should be attempted based on the error.

#### type Auth

```go
type Auth struct {
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	Expires      int64  `json:"expires_in"`
}
```

Auth represents an OAuth2 authentication token.

#### type MimeBody

```go
type MimeBody struct {
	FieldName string
	FileName  string
	Source    io.ReadCloser
	AddFields map[string]string
	Limit     int64
}
```

MimeBody represents a file part for multipart form data uploads.

#### type PostForm

```go
type PostForm map[string]interface{}
```

PostForm is a map for constructing form-encoded request payloads.

#### type PostJSON

```go
type PostJSON map[string]interface{}
```

PostJSON is a map for constructing JSON request payloads.

#### type Query

```go
type Query map[string]interface{}
```

Query is a map for constructing URL query parameters.

#### type TokenStore

```go
type TokenStore interface {
	Save(username string, auth *Auth) error
	Load(username string) (*Auth, error)
	Delete(username string) error
}
```

TokenStore defines the interface for persisting authentication tokens.

#### func  DecodeJSON

```go
func DecodeJSON(resp *http.Response, output interface{}) (err error)
```
DecodeJSON decodes a JSON response body into the given output value.

#### func  IsAPIError

```go
func IsAPIError(err error, code ...string) bool
```
IsAPIError reports whether err is an APIError. If codes are provided, it also
checks whether the error contains any of the specified codes.

#### func  KVLiteStore

```go
func KVLiteStore(store kvlite.Store) *kvLiteStore
```
KVLiteStore returns a TokenStore backed by the given kvlite.Store.

#### func  PrefixAPIError

```go
func PrefixAPIError(prefix string, err error) error
```
PrefixAPIError adds a prefix to an APIError. Returns the original error
unchanged if it is not an APIError.

#### func  SetParams

```go
func SetParams(vars ...interface{}) (output []interface{})
```
SetParams organizes variadic parameters into a normalized slice of Query,
PostJSON, PostForm, and MimeBody values, merging duplicates.
