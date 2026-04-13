# snugforge

A collection of reusable Go utility packages for building terminal-based CLI applications. Snugforge is a library — there is no main package. Consumers import individual packages as needed.

```
go get github.com/cmcoffee/snugforge
```

Requires **Go 1.24+**

## Packages

| Package | Import Path | Description |
|---------|-------------|-------------|
| [nfo](#nfo) | `snugforge/nfo` | Central logging, graceful shutdown, progress bars, user input |
| [eflag](#eflag) | `snugforge/eflag` | Enhanced flag parser with aliases, headers, and multi-values |
| [cfg](#cfg) | `snugforge/cfg` | INI-style config file parser with type-safe getters |
| [kvlite](#kvlite) | `snugforge/kvlite` | Key-value storage with BoltDB and in-memory backends |
| [xsync](#xsync) | `snugforge/xsync` | Concurrency primitives: LimitGroup and atomic BitFlag |
| [wrotate](#wrotate) | `snugforge/wrotate` | io.WriteCloser with automatic size-based file rotation |
| [iotimeout](#iotimeout) | `snugforge/iotimeout` | Timeout-wrapped io.Reader and io.ReadCloser |
| [csvp](#csvp) | `snugforge/csvp` | Callback-based CSV row processor |
| [mimebody](#mimebody) | `snugforge/mimebody` | MIME multipart/form-data encoder for HTTP requests |
| [swapreader](#swapreader) | `snugforge/swapreader` | io.Reader that switches between a byte slice and a reader |
| [jwcrypt](#jwcrypt) | `snugforge/jwcrypt` | JWK key parsing, RSA private key loading, and JWT RS256/RS512 signing |
| [logtime](#logtime) | `snugforge/logtime` | Log timestamp parsing and time-windowed log scanning |
| [logtail](#logtail) | `snugforge/logtail` | Continuous log file tailing with pattern-matched callbacks |
| [xpect](#xpect) | `snugforge/xpect` | Expect-like automation for interactive CLI programs |
| [apiclient](#apiclient) | `snugforge/apiclient` | HTTP API client with OAuth2 auth, retries, rate limiting, and pagination |

---

### nfo

Central logging system with 10 log levels, file rotation, syslog export, graceful shutdown, progress bars, and interactive user input.

```go
import "github.com/cmcoffee/snugforge/nfo"
```

**Log Levels**

Levels are bit flags and can be combined with bitwise OR.

| Constant | Description |
|----------|-------------|
| `INFO` | Informational messages (stdout) |
| `ERROR` | Error messages (prefixed `[ERROR]`) |
| `WARN` | Warning messages (prefixed `[WARN]`) |
| `NOTICE` | Notice messages (prefixed `[NOTICE]`) |
| `DEBUG` | Debug messages (disabled by default) |
| `TRACE` | Trace messages (disabled by default) |
| `FATAL` | Fatal messages (triggers shutdown) |
| `AUX` – `AUX4` | Auxiliary log channels |
| `STD` | All levels except DEBUG and TRACE |
| `ALL` | All levels including DEBUG and TRACE |

**Logging**

```go
nfo.Log("server started on port %d", 8080)
nfo.Err("connection failed: %v", err)
nfo.Warn("disk usage at %d%%", 90)
nfo.Notice("config reloaded")
nfo.Debug("request payload: %s", body)
nfo.Trace("entering function X")
nfo.Fatal("unrecoverable error: %v", err)   // triggers graceful shutdown
```

**Output Control**

```go
nfo.SetOutput(nfo.DEBUG, os.Stderr)   // redirect DEBUG to stderr
nfo.SetPrefix(nfo.ERROR, "[ERR] ")    // change prefix
nfo.ShowTS(nfo.INFO)                  // enable timestamps
nfo.HideTS(nfo.INFO)                  // disable timestamps
nfo.SetTZ(time.UTC)                   // set timestamp timezone
nfo.Stdout("direct to stdout")       // bypass log levels
nfo.Stderr("direct to stderr")
nfo.Flash("temporary status...")      // overwrite-in-place status line
```

**File Logging**

```go
nfo.LogFile("app.log", 10*1024*1024, 5)  // 10MB max, 5 rotations
nfo.SetFile(nfo.ERROR, "errors.log", 5*1024*1024, 3)
```

**Syslog Export**

```go
nfo.EnableExport("udp", "localhost:514", "myapp")
nfo.DisableExport()
```

**Graceful Shutdown**

```go
// Register cleanup functions (executed LIFO on exit)
unreg := nfo.Defer(func() error {
    db.Close()
    return nil
})
defer unreg()  // optionally unregister

// Protect in-flight operations from premature shutdown
nfo.BlockShutdown()
defer nfo.UnblockShutdown()

nfo.Exit(0)                   // trigger deferred shutdown
nfo.ShutdownInProgress()      // check if shutting down
nfo.SetSignals(syscall.SIGINT, syscall.SIGTERM)  // configure signals
```

**Progress Bars & Transfer Monitoring**

```go
monitor := nfo.NewTransferMonitor("downloading", totalBytes, nfo.LeftToRight)
n, err := io.Copy(dst, monitor.Reader(src))
monitor.Done()

counter := nfo.TransferCounter()  // track cumulative transfer size
```

**User Input**

```go
name := nfo.GetInput("Enter your name: ")
pass := nfo.GetSecret("Password: ")
yes := nfo.GetConfirm("Continue?")
nfo.PressEnter("Press Enter to continue...")
name = nfo.NeedAnswer("Name: ", nfo.GetInput)  // loop until non-empty

// Interactive options menu
opts := nfo.NewOptions("Settings", "Selection", 'x')
opts.String("Username", "", "Enter your username", false)
opts.Secret("Password", "", "Enter your password")
opts.Toggle("Debug Mode", false)
opts.Int("Max Retries", 3, "Number of retries", 1, 10)
opts.StringSelect("Environment", "staging", "dev", "staging", "prod")
opts.Func("Run Setup", setupHandler)

// Conditional visibility — only show when a condition is met
opts.Toggle("Verbose Logging", false)
opts.ShowWhen(func() bool { return debugEnabled })

opts.Select(false)  // false = don't separate last item
```

**Utility**

```go
nfo.HumanSize(1536000)  // "1.5MB"
```

---

### eflag

Enhanced wrapper around Go's `flag` package. Adds flag aliases, usage headers/footers, multi-valued flags, and improved formatting.

```go
import "github.com/cmcoffee/snugforge/eflag"
```

**Basic Usage**

```go
debug := eflag.Bool("debug", false, "Enable debug mode.")
eflag.Shorten("debug", 'd')  // alias: -d for --debug

name := eflag.String("name", "", "Your name.")
eflag.Header("MyApp v1.0 - A sample application")
eflag.Footer("Report bugs to bugs@example.com")

eflag.Parse()
```

**Multi-Valued Flags**

```go
var tags []string
eflag.MultiVar(&tags, "tag", "Tags (comma-separated).")
eflag.Parse()
// --tag=a,b,c  →  tags = ["a", "b", "c"]
```

**Argument Reordering**

```go
eflag.AdaptArgs = true  // allow flags after positional args
eflag.InlineArgs("[file ...]", "Files to process.")
eflag.Parse()
```

**Error Handling**

```go
fs := eflag.NewFlagSet("subcmd", eflag.ExitOnError)
// Also: ContinueOnError, PanicOnError, ReturnErrorOnly
```

**Introspection**

```go
eflag.IsSet("debug")          // true if --debug was provided
eflag.ResolveAlias("d")       // "debug"
eflag.NFlag()                 // number of flags set
eflag.NArg()                  // number of remaining args
eflag.Args()                  // remaining args after flags
```

---

### cfg

INI-style configuration file parser with sections, multi-value keys, and type-safe getters. Thread-safe for concurrent access.

```go
import "github.com/cmcoffee/snugforge/cfg"
```

**Config File Format**

```ini
# This is a comment
[server]
host = localhost
port = 8080
debug = true

[database]
hosts = db1.local,
        db2.local,
        db3.local
```

**Reading**

```go
var config cfg.Store
config.File("app.conf")

host := config.Get("server", "host")            // "localhost"
port := config.GetInt("server", "port")          // 8080
debug := config.GetBool("server", "debug")       // true
hosts := config.MGet("database", "hosts")        // ["db1.local", "db2.local", "db3.local"]
joined := config.SGet("database", "hosts")       // "db1.local, db2.local, db3.local"
```

**Writing**

```go
config.Set("server", "host", "0.0.0.0")
config.Unset("server", "debug")
config.Save()        // preserves formatting and comments
config.TrimSave()    // save without preserving original formatting
```

**Introspection**

```go
config.Sections()                    // ["server", "database"]
config.Keys("server")               // ["host", "port", "debug"]
config.Exists("server", "host")     // true
```

**Validation**

```go
// Parse defaults, then validate the config file has required keys
config.Defaults("[required]\nkey1 = default_value")
err := config.Sanitize()  // error if required sections/keys are missing
```

---

### kvlite

Key-value storage with an interface-based design. Ships with a BoltDB-backed persistent store and an in-memory store. Supports optional AES-CFB encryption and hierarchical namespaces.

```go
import "github.com/cmcoffee/snugforge/kvlite"
```

**Opening a Store**

```go
// Persistent (BoltDB)
store, err := kvlite.Open("app.db")

// Persistent with encryption
store, err := kvlite.Open("app.db", "my-secret-key")

// In-memory
store := kvlite.MemStore()
```

**Store Interface**

```go
// Write
err := store.Set("users", "alice", User{Name: "Alice", Age: 30})

// Read
var user User
found, err := store.Get("users", "alice", &user)

// Encrypted write
err = store.CryptSet("users", "alice", sensitiveData)

// Delete
err = store.Unset("users", "alice")

// List
tables, _ := store.Tables()
keys, _ := store.Keys("users")
count, _ := store.CountKeys("users")

// Drop entire table
err = store.Drop("users")
```

**Table Interface**

Provides a focused view on a single table, omitting the table name from every call.

```go
users := store.Table("users")

err := users.Set("bob", User{Name: "Bob"})
found, err := users.Get("bob", &user)
keys, _ := users.Keys()
err = users.Drop()
```

**Namespaces**

```go
sub := store.Sub("tenant-a")     // isolated namespace
bucket := store.Bucket("shared") // shared namespace
```

**Error Handling**

```go
if err == kvlite.ErrLocked {
    // database in use by another instance
}
if err == kvlite.ErrBadPadlock {
    // wrong encryption key
}
```

---

### xsync

Concurrency primitives for thread-safe operations.

```go
import "github.com/cmcoffee/snugforge/xsync"
```

**LimitGroup**

A `sync.WaitGroup` combined with a concurrency limiter. Prevents unbounded goroutine creation.

```go
lg := xsync.NewLimitGroup(10)  // max 10 concurrent goroutines

for _, item := range items {
    lg.Add(1)
    go func(it Item) {
        defer lg.Done()
        process(it)
    }(item)
}
lg.Wait()

// Non-blocking attempt
if lg.Try() {
    go func() {
        defer lg.Done()
        process(item)
    }()
}
```

**BitFlag**

Atomic bit flag operations using compare-and-swap. Lock-free and thread-safe.

```go
const (
    Running  = 1 << iota  // 1
    Paused                // 2
    Stopping              // 4
)

var state xsync.BitFlag

state.Set(Running)
state.Has(Running)    // true
state.Unset(Running)
state.Set(Paused)

// Switch returns the first matching flag
match := state.Switch(Running, Paused, Stopping)  // returns Paused
```

---

### wrotate

`io.WriteCloser` with automatic size-based file rotation and configurable retention.

```go
import "github.com/cmcoffee/snugforge/wrotate"
```

```go
// Rotate at 10MB, keep 5 previous files
w, err := wrotate.OpenFile("app.log", 10*1024*1024, 5)
if err != nil {
    log.Fatal(err)
}
defer w.Close()

// Use as any io.Writer
fmt.Fprintln(w, "log entry")

// Files: app.log → app.log.1 → app.log.2 → ... → app.log.5
// Oldest beyond retention limit is deleted
```

Rotation happens in the background — writes continue to an in-memory buffer during file rotation, so callers are never blocked.

Pass `maxBytes <= 0` or `maxRotations <= 0` to disable rotation and open a plain file.

---

### iotimeout

Wraps `io.Reader` and `io.ReadCloser` with configurable per-read timeouts.

```go
import "github.com/cmcoffee/snugforge/iotimeout"
```

```go
// Wrap a reader with a 30-second timeout
r := iotimeout.NewReader(conn, 30*time.Second)

// Wrap a ReadCloser
rc := iotimeout.NewReadCloser(resp.Body, 10*time.Second)
defer rc.Close()

buf := make([]byte, 4096)
n, err := rc.Read(buf)
if err == iotimeout.ErrTimeout {
    // read timed out
}
```

A timeout of `<= 0` disables the timeout (unlimited wait).

---

### csvp

Callback-based CSV row processor with error type discrimination and comment filtering.

```go
import "github.com/cmcoffee/snugforge/csvp"
```

```go
reader := csvp.NewReader()

reader.Processor = func(row []string) error {
    fmt.Printf("Name: %s, Age: %s\n", row[0], row[1])
    return nil
}

reader.ErrorHandler = func(line int, row string, err error) bool {
    if csvp.IsReadError(err) {
        fmt.Printf("CSV parse error on line %d: %v\n", line, err)
    } else if csvp.IsRowError(err) {
        fmt.Printf("Processing error on line %d: %v\n", line, err)
    }
    return false  // return true to abort
}

file, _ := os.Open("data.csv")
defer file.Close()
reader.Read(file)
```

Lines starting with `#` are treated as comments and skipped.

---

### mimebody

Converts HTTP request bodies to `multipart/form-data` with optional byte-limit support for file uploads. Operates in a streaming fashion for memory efficiency.

```go
import "github.com/cmcoffee/snugforge/mimebody"
```

```go
// Add form fields to an existing request body
fields := map[string]string{"name": "report", "type": "csv"}
err := mimebody.ConvertForm(req, "data", fields)

// File upload with byte limit
err = mimebody.ConvertFormFile(req, "file", "upload.zip", fields, 50*1024*1024)
```

Both functions modify the request in-place: they set the `Content-Type` header and replace `request.Body` with a streaming multipart reader.

---

### swapreader

Minimal `io.Reader` implementation that can switch between reading from a byte slice and an underlying `io.Reader`.

```go
import "github.com/cmcoffee/snugforge/swapreader"
```

```go
r := new(swapreader.Reader)

// Read from bytes
r.SetBytes([]byte("hello world"))
buf := make([]byte, 5)
n, _ := r.Read(buf)  // buf = "hello", n = 5

// Switch to an io.Reader
r.SetReader(os.Stdin)
n, _ = r.Read(buf)    // reads from stdin
```

---

### jwcrypt

JWK key parsing (RFC 7517), RSA private key loading, and JWT RS256/RS512 signing (RFC 7515/7519).

```go
import "github.com/cmcoffee/snugforge/jwcrypt"
```

**Parse a JWK**

```go
jwk, err := jwcrypt.ParseJWK(jsonData)
// Access standard JWK attributes
fmt.Println(jwk.KeyID)      // "kid" field
fmt.Println(jwk.Algorithm)  // "alg" field
fmt.Println(jwk.Use)        // "use" field (sig, enc)
fmt.Println(jwk.KeyType)    // "kty" field

// Use the extracted RSA private key
key := jwk.PrivateKey
```

**Parse an RSA Private Key (auto-detect format)**

```go
// Auto-detects JWK vs PEM/PKCS8 format
key, err := jwcrypt.ParseRSAPrivateKey(keyData)

// With passphrase for encrypted PKCS8
key, err := jwcrypt.ParseRSAPrivateKey(keyData, []byte("secret"))
```

**Sign a JWT**

```go
claims := map[string]interface{}{
    "iss": "my-app",
    "sub": "user@example.com",
    "exp": time.Now().Add(5 * time.Minute).Unix(),
}

// RS256 (RSA SHA-256)
token, err := jwcrypt.SignRS256(key, claims)

// RS512 (RSA SHA-512)
token, err := jwcrypt.SignRS512(key, claims)

// Generic signing with algorithm selection
token, err := jwcrypt.SignJWT(jwcrypt.RS256, key, claims)

// With custom header fields
token, err := jwcrypt.SignRS256(key, claims, map[string]string{"kid": "key-id-123"})
```

Claims can be `map[string]interface{}` or any struct that marshals to JSON.

---

### logtime

Log timestamp parser supporting 20+ formats with auto-detection, and a time-windowed log scanner for extracting entries within a time range.

```go
import "github.com/cmcoffee/snugforge/logtime"
```

**Timestamp Parsing**

```go
// Parse auto-detects the format from 20+ common layouts
t, err := logtime.Parse("2026-03-05T09:01:20+00:00")

// Panics on failure (useful for tests/constants)
t = logtime.MustParse("2026-03-05 03:28:19")

// Register custom formats
logtime.Register("2006/01/02 15:04")
```

Supported formats include ISO 8601, RFC 3339, RFC 1123, Apache CLF, syslog, ANSIC/Unix, and space-separated date-time variants with optional timezone and sub-second precision.

**Time-Windowed Log Scanning**

```go
start, _ := logtime.Parse("2026-03-05 00:00:00")
stop, _ := logtime.Parse("2026-03-05 23:59:59")

file, _ := os.Open("app.log")
defer file.Close()

scanner, err := logtime.NewScanner(start, stop, file)
if err != nil {
    log.Fatal(err)
}

for scanner.Scan() {
    fmt.Printf("[%s] %s\n", scanner.Time(), scanner.Text())
}
if err := scanner.Err(); err != nil {
    log.Fatal(err)
}
```

The scanner auto-detects the timestamp format and prefix length from the first timestamped line. Continuation lines (lines without a timestamp) are grouped with their preceding entry. Only entries with timestamps in `[start, stop]` are returned.

---

### logtail

Continuous log file tailing with pattern-matched callbacks. Watches a log file for new content, parses timestamps using the `logtime` package, and fires registered callbacks when lines match regular expression patterns. Handles file rotation and truncation automatically.

```go
import "github.com/cmcoffee/snugforge/logtail"
```

**Basic Usage**

```go
tail := logtail.Open("/var/log/app.log")

// Register pattern callbacks (regex matched against timestamp-stripped text)
tail.MustOn(`ERROR (.+)`, func(m logtail.Match) {
    fmt.Printf("[%s] error: %s\n", m.Time, m.Groups[0])
})

tail.MustOn(`request completed in (\d+)ms`, func(m logtail.Match) {
    fmt.Printf("latency: %sms\n", m.Groups[0])
})

// Blocks until Close is called or an unrecoverable error occurs
go func() {
    if err := tail.Run(); err != nil {
        log.Fatal(err)
    }
}()

// Stop tailing (safe from any goroutine)
tail.Close()
```

**Options**

```go
tail.SetInterval(500 * time.Millisecond)  // poll interval (default 250ms)
tail.FromStart()                           // process existing content instead of seeking to end
```

**Match Fields**

| Field | Description |
|-------|-------------|
| `Time` | Parsed timestamp of the log entry |
| `Text` | Log line with timestamp prefix stripped |
| `Full` | Original complete log line |
| `Groups` | Capture group matches from the pattern |

Timestamp format is auto-detected from the first timestamped line. Entries at or before the last seen timestamp are skipped to prevent reprocessing after rotation.

---

### xpect

Expect-like automation for interactive command-line programs. Spawn a process, send input, and wait for expected output patterns with configurable timeouts.

```go
import "github.com/cmcoffee/snugforge/xpect"
```

**Basic Usage**

```go
sess, err := xpect.Command("ssh", "user@host")
if err != nil {
    log.Fatal(err)
}
defer sess.Close()

// Wait for password prompt and send password
match, err := sess.Expect("password:")
if err != nil {
    log.Fatal(err)
}
sess.SendLine("my-password")

// Wait for shell prompt
sess.Expect("\\$")
sess.SendLine("ls -la")
sess.Expect("\\$")

// Wait for process to exit
sess.SendLine("exit")
sess.Wait()
```

**Timeouts**

```go
sess.SetTimeout(10 * time.Second)                        // set default timeout
match, err := sess.ExpectTimeout("ready", 5*time.Second) // per-call timeout
err = sess.ExpectEOFTimeout(30 * time.Second)            // wait for EOF
```

**Pattern Matching**

`Expect` uses regular expressions. Capture groups are available in the result:

```go
match, _ := sess.Expect(`version (\d+\.\d+)`)
fmt.Println(match.Before)    // output before the match
fmt.Println(match.Full)      // full matched text
fmt.Println(match.Groups[0]) // first capture group
```

**Interactive Mode**

```go
// Hand control to the user for live interaction
sess.Interact(os.Stdin, os.Stdout)

// Or interact until a pattern matches, then resume scripting
match, _ := sess.InteractUntil(os.Stdin, os.Stdout, "logout")
```

**Logging**

```go
sess.Log = os.Stdout   // watch session output in real time
sess.SendLog = true     // also log Send/SendLine calls
sess.SendMask = "***"   // mask the next Send (for passwords)
```

---

### apiclient

HTTP API client with OAuth2 authentication, automatic retries with exponential backoff, rate limiting, pagination, and pluggable error scanning. Token lifecycle management is built in — the client handles acquisition, refresh, and encrypted storage.

```go
import "github.com/cmcoffee/snugforge/apiclient"
```

**Basic Setup**

```go
client := new(apiclient.APIClient)
client.Server = "api.example.com"
client.VerifySSL = true
client.RequestTimeout = 30 * time.Second
client.ConnectTimeout = 10 * time.Second
client.Retries = 3

// OAuth2 setup
client.ApplicationID = "my-app-id"
client.ClientSecret("my-secret")
client.SetDatabase(db)  // kvlite.Store for token persistence
client.NewToken = func(username string) (*apiclient.Auth, error) {
    // Acquire initial token (password grant, auth code, etc.)
    return &apiclient.Auth{
        AccessToken:  "...",
        RefreshToken: "...",
        Expires:      time.Now().Add(time.Hour).Unix(),
    }, nil
}

// Or use a static API key instead of OAuth2
client.StaticToken = "my-api-key"

// Or use a fully custom auth function (overrides all other auth)
client.AuthFunc = func(req *http.Request) {
    req.Header.Set("X-API-Key", "my-key")
}

// Use HTTP instead of HTTPS (defaults to "https")
client.URLScheme = "http"
```

**Making Requests**

```go
var result MyResponse
err := client.Call(apiclient.APIRequest{
    Username: "user@example.com",
    Method:   http.MethodGet,
    Path:     "/api/v1/users",
    Params:   apiclient.SetParams(apiclient.Query{"limit": 50}),
    Output:   &result,
})

// POST with JSON body
err = client.Call(apiclient.APIRequest{
    Username: "user@example.com",
    Method:   http.MethodPost,
    Path:     "/api/v1/users",
    Params:   apiclient.SetParams(apiclient.PostJSON{"name": "Alice", "role": "admin"}),
    Output:   &result,
})

// File upload with multipart form data
err = client.Call(apiclient.APIRequest{
    Username: "user@example.com",
    Method:   http.MethodPost,
    Path:     "/api/v1/upload",
    Params:   apiclient.SetParams(apiclient.MimeBody{
        FieldName: "file",
        FileName:  "report.csv",
        Source:    file,
        AddFields: map[string]string{"description": "Monthly report"},
    }),
})
```

**Pagination**

```go
var users []User
err := client.PageCall(apiclient.APIRequest{
    Username: "user@example.com",
    Method:   http.MethodGet,
    Path:     "/api/v1/users",
    Output:   &users,
}, 0, 100)  // offset, limit per page
```

**Rate Limiting**

```go
client.SetLimiter(10)          // max 10 concurrent API calls
client.SetTransferLimiter(5)   // max 5 concurrent file transfers
```

**Error Handling**

```go
client.RetryErrorCodes = []string{"ERR_INTERNAL_SERVER_ERROR", "HTTP_STATUS_503"}
client.TokenErrorCodes = []string{"ERR_INVALID_GRANT", "ERR_AUTH_UNAUTHORIZED"}

// Custom error scanner for API-specific error formats
client.ErrorScanner = func(body []byte) apiclient.APIError {
    var e apiclient.APIError
    var resp struct {
        Error   string `json:"error"`
        Message string `json:"message"`
    }
    if json.Unmarshal(body, &resp) == nil && resp.Error != "" {
        e.Register(resp.Error, resp.Message)
    }
    return e
}

// Check error types
if apiclient.IsAPIError(err, "NOT_FOUND") {
    // handle 404
}
err = apiclient.PrefixAPIError("user lookup", err)
```

**Streaming & Raw Responses**

```go
// SendRawRequest returns the response without consuming the body —
// use for SSE, chunked JSON, or large downloads
req, _ := client.NewRequest(http.MethodGet, "/api/v1/events/stream")
resp, err := client.SendRawRequest("user@example.com", req)
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
// read resp.Body incrementally...
```

**Context Support**

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

req, _ := client.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/status")
resp, err := client.SendRequest("user@example.com", req)
```

**Custom Retry Logic**

```go
retry := client.InitRetry("user@example.com", "fetch user list")
for {
    result, err := doSomething()
    if !retry.CheckForRetry(err) {
        break
    }
}
```

---

## Build & Development

```bash
go build ./...          # build all packages
go vet ./...            # vet all packages
gofmt -s -w .           # format code
go test ./...           # run tests
```

## License

MIT
