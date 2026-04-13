/*
Package apiclient provides a reusable HTTP API client with OAuth2 authentication,
automatic retries, rate limiting, pagination, and pluggable error scanning.

It handles token lifecycle management (acquisition, refresh, storage), request
building with JSON/form/multipart payloads, and exponential backoff on transient
failures. Token storage is pluggable via the TokenStore interface, with a default
implementation backed by kvlite.
*/
package apiclient

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cmcoffee/snugforge/iotimeout"
	"github.com/cmcoffee/snugforge/kvlite"
	"github.com/cmcoffee/snugforge/mimebody"
	"github.com/cmcoffee/snugforge/nfo"
	"github.com/cmcoffee/snugforge/xsync"
)

// APIClient is an HTTP API client with OAuth2 authentication, retries, and rate limiting.
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
	db             kvlite.Store                         // Key-value store for token persistence.
	Config         apiConfig                            // Encrypted in-memory config (e.g., client secret).
	limiter        chan struct{}                         // API call rate limiter.
	trans_limiter  chan struct{}                         // File transfer rate limiter.
	NewToken       func(username string) (*Auth, error)  // Callback to acquire a new access token.
	ErrorScanner   func(body []byte) APIError            // Custom response error parser.
	RetryErrorCodes []string                             // Error codes that trigger a retry.
	TokenErrorCodes []string                             // Error codes indicating an invalid token.
	token_lock     sync.Mutex                           // Serializes token operations.
	running        bool                                 // Indicates client has made at least one authenticated call.
	httpClient     *http.Client                         // Shared HTTP client for connection reuse.
	clientOnce     sync.Once                            // Ensures HTTP client is initialized once.
}

// Bitmask constants for retry engine error classification.
const (
	_isRetryError = 1 << iota
	_isTokenError
)

// APIRetryEngine manages retry logic for API calls with exponential backoff.
type APIRetryEngine struct {
	api                     *APIClient
	attempt                 uint
	uid                     string
	user                    string
	task                    string
	addtl_retry_error_codes []string
}

// InitRetry creates a new APIRetryEngine for the given user and task.
// Additional error codes can trigger retries beyond those in RetryErrorCodes.
func (s *APIClient) InitRetry(username string, task_description string, addtl_retry_error_codes ...string) *APIRetryEngine {
	return &APIRetryEngine{
		s,
		0,
		string(randBytes(8)),
		username,
		task_description,
		addtl_retry_error_codes,
	}
}

// CheckForRetry determines whether a retry should be attempted based on the error.
// It considers retry policies, error types, and the retry limit.
func (a *APIRetryEngine) CheckForRetry(err error) bool {
	var flag xsync.BitFlag

	if err == nil {
		if a.attempt > 0 {
			nfo.Debug("[#%s]: %s -> %v: success!! (retry %d/%d)", a.uid, a.user, a.task, a.attempt, a.api.Retries)
		}
		return false
	}

	if a.attempt > a.api.Retries {
		nfo.Debug("[#%s] %s -> %v: %s (exhausted retries)", a.uid, a.user, a.task, err.Error())
		return false
	}

	if !isBlank(a.user) && a.api.isTokenError(a.user, err) {
		flag.Set(_isTokenError)
		flag.Set(_isRetryError)
	} else {
		if a.api.isRetryError(err) || !IsAPIError(err) || (len(a.addtl_retry_error_codes) > 0 && IsAPIError(err, a.addtl_retry_error_codes[0:]...)) {
			flag.Set(_isRetryError)
		}
	}

	if flag.Has(_isTokenError | _isRetryError) {
		if a.attempt == 0 {
			if a.api.Retries > 0 {
				nfo.Debug("[#%s] %s -> %s: %s (will retry)", a.uid, a.user, a.task, err.Error())
			}
		} else {
			nfo.Debug("[#%s] %s -> %s: %s (retry %d/%d)", a.uid, a.user, a.task, err.Error(), a.attempt, a.api.Retries)
		}
	}

	if flag.Has(_isRetryError) {
		a.api.BackoffTimer(uint(a.attempt))
		a.attempt++
		return true
	}

	return false
}

// Fulfill executes an HTTP request with retry logic and decodes the JSON response into output.
func (s *APIClient) Fulfill(username string, req *http.Request, output interface{}) (err error) {
	var dont_retry bool

	close_resp := func(resp *http.Response) {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}

	var resp *http.Response

	if req.GetBody == nil && req.Body != nil {
		dont_retry = true
		nfo.Debug("[%s]: Request body is not re-readable; retry disabled for %s.", username, req.URL.Path)
	} else {
		orig_body := req.GetBody
		req.GetBody = func() (io.ReadCloser, error) {
			body, err := orig_body()
			if err != nil {
				return nil, err
			}
			return iotimeout.NewReadCloser(body, s.RequestTimeout), nil
		}
	}

	retry := s.InitRetry(username, req.URL.Path)

	for {
		if req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				return err
			}
		}

		resp, err = s.SendRequest(username, req)

		if err == nil && resp != nil {
			err = DecodeJSON(resp, output)
		}

		if retry.CheckForRetry(err) {
			if !dont_retry {
				close_resp(resp)
				continue
			}
		}
		close_resp(resp)
		return err
	}
}

// SetDatabase sets the kvlite store and initializes the default TokenStore.
func (s *APIClient) SetDatabase(db kvlite.Store) {
	s.db = db
	s.TokenStore = KVLiteStore(db.Sub("tokens"))
}

// SetLimiter configures the rate limiter for API calls.
func (s *APIClient) SetLimiter(max_calls int) {
	if max_calls <= 0 {
		max_calls = 1
	}
	if s.limiter == nil {
		s.limiter = make(chan struct{}, max_calls)
	}
}

// GetLimit returns the current API rate limit capacity.
func (s *APIClient) GetLimit() int {
	if s.limiter != nil {
		return cap(s.limiter)
	}
	return 1
}

// SetTransferLimiter configures the concurrent transfer limiter.
func (s *APIClient) SetTransferLimiter(max_transfers int) {
	if max_transfers <= 0 {
		max_transfers = 1
	}
	if s.trans_limiter == nil {
		s.trans_limiter = make(chan struct{}, max_transfers)
	}
}

// GetTransferLimit returns the transfer limiter capacity.
func (s *APIClient) GetTransferLimit() int {
	if s.trans_limiter != nil {
		return cap(s.trans_limiter)
	}
	return 1
}

// TokenStore defines the interface for persisting authentication tokens.
type TokenStore interface {
	Save(username string, auth *Auth) error
	Load(username string) (*Auth, error)
	Delete(username string) error
}

// kvLiteStore implements TokenStore using a kvlite.Table.
type kvLiteStore struct {
	table kvlite.Table
}

// KVLiteStore returns a TokenStore backed by the given kvlite.Store.
func KVLiteStore(store kvlite.Store) *kvLiteStore {
	return &kvLiteStore{store.Table("tokens")}
}

// Save persists the authentication token for the given username (encrypted).
func (t *kvLiteStore) Save(username string, auth *Auth) error {
	return t.table.CryptSet(username, &auth)
}

// Load retrieves the authentication token for the given username.
func (t *kvLiteStore) Load(username string) (*Auth, error) {
	var auth *Auth
	_, err := t.table.Get(username, &auth)
	return auth, err
}

// Delete removes the authentication token for the given username.
func (t *kvLiteStore) Delete(username string) error {
	return t.table.Unset(username)
}

// apiConfig holds encrypted in-memory configuration values.
type apiConfig struct {
	key        []byte
	config_map map[string][]byte
}

func (k *apiConfig) Set(key, value string) {
	if k.config_map == nil {
		k.config_map = make(map[string][]byte)
	}
	k.config_map[key] = k.encrypt(value)
}

func (k *apiConfig) Get(key string) string {
	if k.config_map == nil {
		k.config_map = make(map[string][]byte)
	}
	if v, ok := k.config_map[key]; ok {
		return k.decrypt(v)
	}
	return ""
}

func (k *apiConfig) encrypt(input string) []byte {
	if k.key == nil {
		k.key = make([]byte, 32)
		rand.Read(k.key)
	}

	block, err := aes.NewCipher(k.key)
	if err != nil {
		nfo.Fatal(err)
	}
	in_bytes := []byte(input)

	buff := make([]byte, len(in_bytes))
	copy(buff, in_bytes)

	cipher.NewCFBEncrypter(block, k.key[0:block.BlockSize()]).XORKeyStream(buff, buff)

	return buff
}

func (k *apiConfig) decrypt(input []byte) string {
	if k.key == nil {
		return ""
	}

	output := make([]byte, len(input))

	block, _ := aes.NewCipher(k.key)
	cipher.NewCFBDecrypter(block, k.key[0:block.BlockSize()]).XORKeyStream(output, input)

	return string(output)
}

// GetClientSecret retrieves the decrypted client secret.
func (s *APIClient) GetClientSecret() string {
	return s.Config.Get("client_secret")
}

// ClientSecret stores the client secret (encrypted in memory).
func (s *APIClient) ClientSecret(client_secret_key string) {
	s.Config.Set("client_secret", client_secret_key)
}

// APIRequest represents a structured API call.
type APIRequest struct {
	Username string
	Version  int
	Header   http.Header
	Method   string
	Path     string
	Params   []interface{}
	Output   interface{}
}

// SetPath is a convenience alias for fmt.Sprintf for building URL paths.
var SetPath = fmt.Sprintf

// PostJSON is a map for constructing JSON request payloads.
type PostJSON map[string]interface{}

// PostForm is a map for constructing form-encoded request payloads.
type PostForm map[string]interface{}

// Query is a map for constructing URL query parameters.
type Query map[string]interface{}

// MimeBody represents a file part for multipart form data uploads.
type MimeBody struct {
	FieldName string
	FileName  string
	Source    io.ReadCloser
	AddFields map[string]string
	Limit     int64
}

// Auth represents an OAuth2 authentication token.
type Auth struct {
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	Expires      int64  `json:"expires_in"`
}

// SetParams organizes variadic parameters into a normalized slice of
// Query, PostJSON, PostForm, and MimeBody values, merging duplicates.
func SetParams(vars ...interface{}) (output []interface{}) {
	if len(vars) == 0 {
		return nil
	}
	var (
		post_json PostJSON
		query     Query
		form      PostForm
		mb        MimeBody
	)

	mb_set := false

	process_vars := func(vars interface{}) {
		switch x := vars.(type) {
		case Query:
			if query == nil {
				query = x
			} else {
				for key, val := range x {
					query[key] = val
				}
			}
		case PostJSON:
			if post_json == nil {
				post_json = x
			} else {
				for key, val := range x {
					post_json[key] = val
				}
			}
		case PostForm:
			if form == nil {
				form = x
			} else {
				for key, val := range x {
					form[key] = val
				}
			}
		case MimeBody:
			mb = x
			mb_set = true
		}
	}

	for {
		tmp := vars[0:0]
		for _, v := range vars {
			switch val := v.(type) {
			case []interface{}:
				for _, elem := range val {
					tmp = append(tmp[0:], elem)
				}
			case nil:
				continue
			default:
				process_vars(val)
			}
		}
		if len(tmp) == 0 {
			break
		}
		vars = tmp
	}

	if post_json != nil {
		output = append(output, post_json)
	}
	if query != nil {
		output = append(output, query)
	}
	if form != nil {
		output = append(output, form)
	}
	if mb_set {
		output = append(output, mb)
	}
	return
}

// SetToken sets the Bearer authorization header for the given user.
// If StaticToken is set, it is used directly. Otherwise, the token is
// loaded from the TokenStore, refreshed if expired, or acquired via
// the NewToken callback.
func (s *APIClient) SetToken(username string, req *http.Request) (err error) {
	// AuthFunc overrides all other auth logic when set.
	if s.AuthFunc != nil {
		s.AuthFunc(req)
		return nil
	}

	// Static token bypasses OAuth2 flow entirely.
	if s.StaticToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.StaticToken)
		return nil
	}

	if s.TokenStore == nil {
		return fmt.Errorf("apiclient: TokenStore not initialized")
	}

	s.token_lock.Lock()
	defer s.token_lock.Unlock()

	token, err := s.TokenStore.Load(username)
	if err != nil {
		return err
	}

	if token != nil {
		if token.Expires <= time.Now().Unix() {
			nfo.Debug("[%s]: Access token expired, using refresh token instead.", username)
			err = s.refreshToken(username, token)
			if err != nil {
				nfo.Debug("[%s]: Unable to use refresh token: %v", username, err)
				if s.running && !s.ReacquireToken {
					nfo.Fatal("Access token has expired, must reauthenticate for new access token.")
				}
				token = nil
				err = nil
			}
		}
	}

	if token == nil {
		if s.NewToken == nil {
			return fmt.Errorf("apiclient: NewToken not initialized")
		}
		s.TokenStore.Delete(username)

		token, err = s.NewToken(username)
		if err != nil {
			return err
		}
		nfo.Debug("[%s]: Acquired new access token.", username)
	}

	if token != nil {
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		if err := s.TokenStore.Save(username, token); err != nil {
			return err
		}
	}

	s.running = true

	return nil
}

// refreshToken obtains a new access token using the OAuth2 refresh token grant.
func (s *APIClient) refreshToken(username string, auth *Auth) error {
	if auth == nil || auth.RefreshToken == "" {
		return fmt.Errorf("no refresh token found for %s", username)
	}
	nfo.Debug("Using refresh token to obtain new token.")

	refresh_path := s.RefreshPath
	if refresh_path == "" {
		refresh_path = "/oauth/token"
	}

	path := fmt.Sprintf("https://%s%s", s.Server, refresh_path)

	req, err := http.NewRequest(http.MethodPost, path, nil)
	if err != nil {
		return err
	}

	http_header := make(http.Header)
	http_header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.AgentString != "" {
		http_header.Set("User-Agent", s.AgentString)
	}

	req.Header = http_header

	postform := &url.Values{
		"client_id":     {s.ApplicationID},
		"client_secret": {s.GetClientSecret()},
		"grant_type":    {"refresh_token"},
		"refresh_token": {auth.RefreshToken},
	}

	nfo.Trace("[%s]: %s", s.Server, username)
	nfo.Trace("--> ACTION: \"POST\" PATH: \"%s\"", path)
	for k, v := range *postform {
		if k == "grant_type" || k == "RedirectURI" || k == "scope" {
			nfo.Trace("\\-> POST PARAM: %s VALUE: %s", k, v)
		} else {
			nfo.Trace("\\-> POST PARAM: %s VALUE: [HIDDEN]", k)
		}
	}

	var new_token struct {
		AccessToken  string      `json:"access_token"`
		Scope        string      `json:"scope"`
		RefreshToken string      `json:"refresh_token"`
		Expires      interface{} `json:"expires_in"`
	}

	req.Body = io.NopCloser(bytes.NewReader([]byte(postform.Encode())))
	req.Body = iotimeout.NewReadCloser(req.Body, s.RequestTimeout)
	defer req.Body.Close()

	resp, err := s.SendRequest("", req)
	if err != nil {
		return err
	}

	if err := DecodeJSON(resp, &new_token); err != nil {
		return err
	}

	if new_token.Expires != nil {
		expiry, _ := strconv.ParseInt(fmt.Sprintf("%v", new_token.Expires), 0, 64)
		auth.Expires = time.Now().Unix() + expiry
	}

	auth.AccessToken = new_token.AccessToken
	auth.RefreshToken = new_token.RefreshToken
	auth.Scope = new_token.Scope

	return nil
}

// spanner converts various input types to a comma-separated string.
func (s *APIClient) spanner(input interface{}) string {
	switch v := input.(type) {
	case []string:
		return strings.Join(v, ",")
	case []int:
		var output []string
		for _, i := range v {
			output = append(output, fmt.Sprintf("%v", i))
		}
		return strings.Join(output, ",")
	default:
		return fmt.Sprintf("%v", input)
	}
}

// readCloser combines an io.Reader with a close function.
type readCloser struct {
	closer func() error
	io.Reader
}

func (r readCloser) Close() error {
	return r.closer()
}

func newReadCloser(src io.Reader, close_func func() error) io.ReadCloser {
	return readCloser{close_func, src}
}

// snoopReader reads at least min bytes from src, returning a reader for the
// initial bytes and a new ReadCloser that replays them before continuing.
func snoopReader(src io.ReadCloser, min int) (snoop_reader io.Reader, output io.ReadCloser, err error) {
	var n int
	buffer := make([]byte, min)

	n, err = io.ReadAtLeast(src, buffer, min)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, src, err
	}

	buffer = buffer[0:n]

	snoop_reader = bytes.NewReader(buffer)

	if n == min {
		output = readCloser{src.Close, io.MultiReader(bytes.NewReader(buffer), src)}
	} else {
		output = readCloser{src.Close, bytes.NewReader(buffer)}
	}

	err = nil

	return
}

// respErrorCheck reads the first 64KB of the response body to scan for errors.
func (s *APIClient) respErrorCheck(resp *http.Response) (err error) {
	var (
		snoop_buffer bytes.Buffer
		snoop_reader io.Reader
	)

	if resp == nil {
		return nil
	}

	snoop_reader, resp.Body, err = snoopReader(iotimeout.NewReadCloser(resp.Body, s.RequestTimeout), 65536)
	if err != nil {
		return err
	}

	snoop_reader = io.TeeReader(snoop_reader, &snoop_buffer)

	msg, err := io.ReadAll(snoop_reader)
	if err != nil {
		return err
	}

	var escanner func(body []byte) APIError

	if s.ErrorScanner != nil {
		escanner = s.ErrorScanner
	} else {
		escanner = func(body []byte) (e APIError) {
			return e
		}
	}

	e := escanner(msg)
	if !e.noError() {
		snoop_response(resp.Status, &snoop_buffer)
		return e
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	snoop_response(resp.Status, &snoop_buffer)

	e.Register(fmt.Sprintf("HTTP_STATUS_%d", resp.StatusCode), resp.Status)
	return e
}

// DecodeJSON decodes a JSON response body into the given output value.
func DecodeJSON(resp *http.Response, output interface{}) (err error) {
	var (
		snoop_buffer bytes.Buffer
		body         io.Reader
	)

	defer resp.Body.Close()

	body = io.TeeReader(resp.Body, &snoop_buffer)
	defer snoop_response(resp.Status, &snoop_buffer)

	msg, err := io.ReadAll(body)

	if output == nil {
		return nil
	}

	if err != nil {
		return err
	}

	if len(msg) > 0 {
		err = json.Unmarshal(msg, output)
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return fmt.Errorf("unable to decode response from %s: %s", resp.Request.URL.Host, err.Error())
		}
	}

	return
}

// snoop_response logs the response status and body at Trace level, redacting tokens.
func snoop_response(respStatus string, body *bytes.Buffer) {
	nfo.Trace("<-- RESPONSE STATUS: %s", respStatus)

	var snoop_generic map[string]interface{}
	dec := json.NewDecoder(body)
	str := body.String()
	if err := dec.Decode(&snoop_generic); err != nil {
		nfo.Trace("<-- RESPONSE BODY: \n%s\n", str)
		return
	}
	if snoop_generic != nil {
		for v := range snoop_generic {
			switch v {
			case "refresh_token":
				fallthrough
			case "access_token":
				snoop_generic[v] = "[HIDDEN]"
			}
		}
	}
	o, _ := json.MarshalIndent(&snoop_generic, "", "  ")
	nfo.Trace("<-- RESPONSE BODY: \n%s\n", string(o))
}

// initHTTPClient initializes the shared HTTP client with connection pooling.
func (s *APIClient) initHTTPClient() {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   s.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   s.ConnectTimeout,
		ResponseHeaderTimeout: s.RequestTimeout,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
	}

	if !s.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if s.ProxyURI != "" {
		proxyURL, err := url.Parse(s.ProxyURI)
		if err != nil {
			nfo.Fatal(err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	s.httpClient = &http.Client{
		Transport: transport,
	}
}

// needsAuth reports whether SetToken should be called. It returns true when
// a username is provided OR when auth is configured without a username
// (AuthFunc or StaticToken).
func (s *APIClient) needsAuth(username string) bool {
	if !isBlank(username) {
		return true
	}
	return s.AuthFunc != nil || s.StaticToken != ""
}

// SendRequest sends an HTTP request, setting the auth token if needed.
// The response body is consumed for error checking; use SendRawRequest for streaming.
func (s *APIClient) SendRequest(username string, req *http.Request) (resp *http.Response, err error) {
	s.clientOnce.Do(s.initHTTPClient)

	if s.needsAuth(username) {
		err = s.SetToken(username, req)
		if err != nil {
			return nil, err
		}
	}

	if req.Body != nil {
		req.Body = iotimeout.NewReadCloser(req.Body, s.RequestTimeout)
	}

	resp, err = s.httpClient.Do(req)
	if err == nil {
		err = s.respErrorCheck(resp)
	}

	return
}

// SendRawRequest sends an HTTP request and returns the raw response without
// reading or error-checking the body. The caller is responsible for closing
// resp.Body. This is intended for streaming responses (SSE, chunked JSON)
// where the body must be read incrementally.
func (s *APIClient) SendRawRequest(username string, req *http.Request) (resp *http.Response, err error) {
	s.clientOnce.Do(s.initHTTPClient)

	if s.needsAuth(username) {
		err = s.SetToken(username, req)
		if err != nil {
			return nil, err
		}
	}

	return s.httpClient.Do(req)
}

// scheme returns the URL scheme for this client ("http" or "https").
func (s *APIClient) scheme() string {
	if s.URLScheme != "" {
		return s.URLScheme
	}
	return "https"
}

// NewRequest creates an HTTP(S) request to the configured server.
func (s *APIClient) NewRequest(method, path string) (req *http.Request, err error) {
	req, err = http.NewRequest(method, fmt.Sprintf("%s://%s%s", s.scheme(), s.Server, path), nil)
	if err != nil {
		return nil, err
	}

	req.URL.Host = s.Server
	req.URL.Scheme = s.scheme()

	if s.AgentString != "" {
		req.Header.Set("User-Agent", s.AgentString)
	}
	req.Header.Set("Referer", s.scheme()+"://"+s.Server+"/")

	return req, nil
}

// NewRequestWithContext creates an HTTP(S) request with a context for cancellation.
func (s *APIClient) NewRequestWithContext(ctx context.Context, method, path string) (req *http.Request, err error) {
	req, err = http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s://%s%s", s.scheme(), s.Server, path), nil)
	if err != nil {
		return nil, err
	}

	req.URL.Host = s.Server
	req.URL.Scheme = s.scheme()

	if s.AgentString != "" {
		req.Header.Set("User-Agent", s.AgentString)
	}
	req.Header.Set("Referer", s.scheme()+"://"+s.Server+"/")

	return req, nil
}

// Call performs a structured API request with rate limiting and retry logic.
func (s *APIClient) Call(api_req APIRequest) (err error) {
	if s.limiter != nil {
		s.limiter <- struct{}{}
		defer func() { <-s.limiter }()
	}

	req, err := s.NewRequest(api_req.Method, api_req.Path)
	if err != nil {
		return err
	}

	nfo.Trace("[%s]: %s", s.Server, api_req.Username)
	nfo.Trace("--> METHOD: \"%s\" PATH: \"%s\"", strings.ToUpper(api_req.Method), api_req.Path)

	var body []byte

	for k, v := range api_req.Header {
		req.Header[k] = v
	}

	for k, v := range req.Header {
		if strings.HasPrefix(v[0], "Bearer") {
			v = []string{"Bearer [HIDDEN]"}
		}
		nfo.Trace("--> HEADER: %s: %s", k, v)
	}

	skip_getBody := false

	for _, in := range api_req.Params {
		switch i := in.(type) {
		case PostForm:
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			nfo.Trace("--> HEADER: Content-Type: [application/x-www-form-urlencoded]")
			p := make(url.Values)
			for k, v := range i {
				p.Add(k, s.spanner(v))
				nfo.Trace("\\-> POST PARAM: \"%s\" VALUE: \"%s\"", k, p[k])
			}
			body = []byte(p.Encode())
		case PostJSON:
			req.Header.Set("Content-Type", "application/json")
			nfo.Trace("--> HEADER: Content-Type: [application/json]")

			json_data, err := json.Marshal(i)
			if err != nil {
				return err
			}

			nfo.Trace("\\-> POST JSON: %s", string(json_data))
			body = json_data
		case Query:
			q := req.URL.Query()
			for k, v := range i {
				q.Set(k, s.spanner(v))
				nfo.Trace("\\-> QUERY: %s=%s", k, q[k])
			}
			req.URL.RawQuery = q.Encode()
		case MimeBody:
			req.Body = i.Source
			mimebody.ConvertFormFile(req, i.FieldName, i.FileName, i.AddFields, i.Limit)
			skip_getBody = true
			nfo.Trace("--> HEADER: Content-Type: [multipart/form-data]")
			for k, v := range i.AddFields {
				nfo.Trace("\\-> FORM FIELD: %s=%s", k, v)
			}
			if !isBlank(i.FileName) {
				nfo.Trace("\\-> FORM DATA: name=\"%s\"; filename=\"%s\"", i.FieldName, i.FileName)
			} else {
				nfo.Trace("\\-> FORM DATA: name=\"%s\"", i.FieldName)
			}
		case nil:
			continue
		default:
			return fmt.Errorf("unknown request parameter type")
		}
	}

	if !skip_getBody {
		req.GetBody = getBodyBytes(body)
	}

	return s.Fulfill(api_req.Username, req, api_req.Output)
}

// BackoffTimer pauses with exponential backoff: (retry+1)^2 seconds.
func (s *APIClient) BackoffTimer(retry uint) {
	if retry < s.Retries {
		wait := (time.Second * time.Duration(retry+1)) * time.Duration(retry+1)
		nfo.Debug("Backoff: waiting %s before retry %d.", wait, retry+1)
		time.Sleep(wait)
	}
}

// PageCall paginates through API responses, accumulating results from
// successive calls with increasing offsets until all data is retrieved.
func (s *APIClient) PageCall(req APIRequest, offset, limit int) (err error) {
	output := req.Output
	params := req.Params

	if limit <= 0 {
		limit = 100
	}

	var managed bool

	if offset < 0 {
		offset = 0
	} else {
		managed = true
	}

	var o struct {
		Data interface{} `json:"data"`
	}

	o.Data = req.Output

	var tmp []map[string]interface{}

	var enc_buff bytes.Buffer
	enc := json.NewEncoder(&enc_buff)
	dec := json.NewDecoder(&enc_buff)

	for {
		req.Params = SetParams(params, Query{"limit": limit, "offset": offset})
		req.Output = &o
		if err = s.Call(req); err != nil {
			return err
		}
		if o.Data != nil {
			enc_buff.Reset()
			err := enc.Encode(o.Data)
			if err != nil {
				return err
			}
			var t []map[string]interface{}
			err = dec.Decode(&t)
			if err != nil {
				return err
			}
			tmp = append(tmp, t[0:]...)
			if len(t) < limit || managed {
				break
			} else {
				nfo.Debug("PageCall %s: Fetched %d records so far (offset %d -> %d).", req.Path, len(tmp), offset, offset+limit)
				offset = offset + limit
			}
		} else {
			return fmt.Errorf("unexpected empty response")
		}
	}

	enc_buff.Reset()

	if err := enc.Encode(tmp); err != nil {
		return err
	} else {
		tmp = nil
		if err = dec.Decode(output); err != nil {
			return err
		}
	}
	return
}

// --- internal helpers ---

// isBlank returns true if any of the given strings are empty.
func isBlank(input ...string) bool {
	for _, v := range input {
		if len(v) == 0 {
			return true
		}
	}
	return false
}

// randBytes generates a random alphanumeric byte slice of the given length.
func randBytes(sz int) []byte {
	if sz <= 0 {
		sz = 16
	}

	ch := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/"
	chlen := len(ch)

	rand_string := make([]byte, sz)
	rand.Read(rand_string)

	for i, v := range rand_string {
		rand_string[i] = ch[v%byte(chlen)]
	}

	return rand_string
}

// getBodyBytes returns a function that produces a fresh ReadCloser from the given bytes.
func getBodyBytes(input []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(input)), nil
	}
}
