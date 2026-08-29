# jwcrypt
--
    import "github.com/cmcoffee/snugforge/jwcrypt"

Package jwcrypt signs and verifies RSA-based JSON Web Tokens, and parses the
keys they are signed with: JWK documents (public or private), JWK Sets, and
PEM/PKCS8 private keys.

It does no network I/O. Fetching an issuer's OIDC discovery document and caching
the keys behind it are application decisions -- which HTTP client, which
timeouts, when a rotation warrants a re-fetch -- so a caller retrieves the JWK
Set itself and hands the bytes to ParseJWKSet.

## Usage

#### func  ParseRSAPrivateKey

```go
func ParseRSAPrivateKey(keyData []byte, passphrase ...[]byte) (*rsa.PrivateKey, error)
```
ParseRSAPrivateKey auto-detects JWK (JSON) vs PEM/PKCS8 format and parses the
RSA private key. Optional passphrase for encrypted PKCS8 keys.

#### func  SignJWT

```go
func SignJWT(alg JWTAlgorithm, key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error)
```
SignJWT creates a signed JWT token (header.payload.signature) using the
specified algorithm (RS256 or RS512). Claims can be map[string]interface{} or a
struct. Optional headerFields adds custom JWT header fields (e.g. "kid",
"type").

#### func  SignRS256

```go
func SignRS256(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error)
```
SignRS256 creates a signed JWT token using RS256 (RSA SHA-256).

#### func  SignRS512

```go
func SignRS512(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error)
```
SignRS512 creates a signed JWT token using RS512 (RSA SHA-512).

#### type Claims

```go
type Claims map[string]interface{}
```

Claims is a decoded JWT payload. The accessors handle the shapes RFC 7519
actually permits, which is where hand-rolled claim checks usually go wrong.

#### func  VerifyJWT

```go
func VerifyJWT(token string, key *rsa.PublicKey, opts VerifyOptions) (Claims, error)
```
VerifyJWT checks a token's signature against key and then its standard claims,
returning the decoded payload. The token's own "alg" header selects a hash only
from the caller's allow-list, and never selects the key type: that is the whole
defence against algorithm-confusion, where a token declaring HS256 (or "none")
talks a verifier out of doing RSA at all.

#### func (Claims) String

```go
func (c Claims) String(name string) string
```
String returns a string claim, or "" if absent or another type.

#### func (Claims) Strings

```go
func (c Claims) Strings(name string) []string
```
Strings returns a claim that RFC 7519 allows to be either a single string or an
array of them (aud is the one that bites). A plain string yields a one-element
slice; anything else yields nil.

#### func (Claims) Time

```go
func (c Claims) Time(name string) (time.Time, bool)
```
Time returns a NumericDate claim (exp, nbf, iat) and whether it was present and
well-formed. JSON numbers decode as float64, so seconds are read that way and
fractional seconds are kept.

#### type JWK

```go
type JWK struct {
	KeyType    string   `json:"kty"`
	Use        string   `json:"use,omitempty"`
	KeyOps     []string `json:"key_ops,omitempty"`
	Algorithm  string   `json:"alg,omitempty"`
	KeyID      string   `json:"kid,omitempty"`
	X5C        []string `json:"x5c,omitempty"` // base64 DER certificate chain, leaf first
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey

	// Extra holds members outside RFC 7517's registry, kept raw. Issuers attach
	// their own (Microsoft's Bot Framework publishes "endorsements"), and a
	// caller that needs one should not have to re-parse the document to see it.
	Extra map[string]json.RawMessage `json:"-"`
}
```

JWK represents a parsed JSON Web Key with all standard attributes (RFC 7517).

A JWK may carry a public key, a private key, or both. PublicKey is populated
whenever the key parses at all; PrivateKey only when the private parameters are
present. An issuer's published signing keys -- the JWK Set behind an OIDC
discovery document -- are public-only, so a caller verifying a token reads
PublicKey and must expect PrivateKey to be nil.

#### func  ParseJWK

```go
func ParseJWK(jwkData []byte) (*JWK, error)
```
ParseJWK parses a JWK (JSON Web Key) and returns a JWK struct with all standard
attributes and the extracted RSA key material.

Public-only keys are supported: when the private parameters are absent the
result carries PublicKey and a nil PrivateKey. That is the normal shape of a key
fetched from an issuer's JWK Set, and rejecting it -- as this function once did,
by unconditionally assembling and validating an rsa.PrivateKey -- left no way to
verify anything this package signed.

#### type JWKSet

```go
type JWKSet struct {
	Keys []*JWK
}
```

JWKSet is a parsed RFC 7517 key set, the document an OIDC issuer publishes at
its jwks_uri.

#### func  ParseJWKSet

```go
func ParseJWKSet(data []byte) (*JWKSet, error)
```
ParseJWKSet parses a {"keys":[...]} document. Keys this package cannot use (EC,
OKP, malformed entries) are skipped rather than failing the document: issuers
mix key types freely, and one unusable entry should not take the usable ones
with it. A set with no usable key at all is an error.

#### func (*JWKSet) KeyByID

```go
func (s *JWKSet) KeyByID(kid string) (*JWK, bool)
```
KeyByID returns the key with the given kid.

#### func (*JWKSet) VerifyJWT

```go
func (s *JWKSet) VerifyJWT(token string, opts VerifyOptions) (Claims, error)
```
VerifyJWT selects the key the token's kid names and verifies against it,
returning UnknownKeyIDError when the set does not hold that key. A token with no
kid is tried against every key in the set, which some small issuers require; the
first key's error is reported when none of them work, since a claim failure is
the same for all of them.

#### type JWTAlgorithm

```go
type JWTAlgorithm string
```

JWT signing algorithm.

```go
const (
	RS256 JWTAlgorithm = "RS256"
	RS512 JWTAlgorithm = "RS512"
)
```

#### type JWTHeader

```go
type JWTHeader struct {
	Algorithm JWTAlgorithm `json:"alg"`
	KeyID     string       `json:"kid,omitempty"`
	Type      string       `json:"typ,omitempty"`
}
```

JWTHeader is a decoded JOSE header.

#### func  DecodeJWTHeader

```go
func DecodeJWTHeader(token string) (JWTHeader, error)
```
DecodeJWTHeader reads a token's header WITHOUT verifying anything. It exists so
a caller can learn which key the token names (kid) before it has a key to check
it with. Nothing it returns is trustworthy until VerifyJWT succeeds.

#### type UnknownKeyIDError

```go
type UnknownKeyIDError struct{ KeyID string }
```

UnknownKeyIDError reports that a token names a key the set does not hold.
Distinct from a verification failure on purpose: an unknown kid usually means
the issuer rotated its keys and the caller should re-fetch the set, whereas a
failed signature means reject the token and move on. Conflating the two gives
you either a bridge that dies at the next rotation or one that re-fetches on
every forged token.

#### func (*UnknownKeyIDError) Error

```go
func (e *UnknownKeyIDError) Error() string
```

#### type VerifyOptions

```go
type VerifyOptions struct {
	// Issuer, when set, must equal the token's iss claim.
	Issuer string
	// Audience, when set, must appear in the token's aud claim.
	Audience string
	// Algorithms is the accepted algorithm allow-list. Empty means RS256+RS512.
	Algorithms []JWTAlgorithm
	// Leeway absorbs clock skew between issuer and verifier when checking exp
	// and nbf. Zero means exact.
	Leeway time.Duration
	// AllowMissingExpiry permits a token with no exp claim. Off by default: an
	// unexpiring bearer token is almost never what the caller meant.
	AllowMissingExpiry bool
	// Now overrides the clock, for tests.
	Now func() time.Time
}
```

VerifyOptions controls what VerifyJWT insists on beyond the signature.
