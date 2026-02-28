# jwcrypt
--
    import "github.com/cmcoffee/snugforge/jwcrypt"


## Usage

#### type JWK

```go
type JWK struct {
	KeyType    string   `json:"kty"`
	Use        string   `json:"use,omitempty"`
	KeyOps     []string `json:"key_ops,omitempty"`
	Algorithm  string   `json:"alg,omitempty"`
	KeyID      string   `json:"kid,omitempty"`
	PrivateKey *rsa.PrivateKey
}
```
JWK represents a parsed JSON Web Key with all standard attributes (RFC 7517).

#### func  ParseJWK

```go
func ParseJWK(jwkData []byte) (*JWK, error)
```
ParseJWK parses a JWK (JSON Web Key) and returns a JWK struct with all
standard attributes and the extracted RSA private key.

#### func  ParseRSAPrivateKey

```go
func ParseRSAPrivateKey(keyData []byte, passphrase ...[]byte) (*rsa.PrivateKey, error)
```
ParseRSAPrivateKey auto-detects JWK (JSON) vs PEM/PKCS8 format and parses the
RSA private key. Optional passphrase for encrypted PKCS8 keys.

#### func  SignRS256

```go
func SignRS256(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error)
```
SignRS256 creates a signed JWT token (header.payload.signature) using RS256.
Claims can be map[string]interface{} or a struct. Optional headerFields adds
custom JWT header fields (e.g. "kid", "type").
