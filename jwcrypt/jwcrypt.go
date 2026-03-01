package jwcrypt

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"

	"github.com/youmark/pkcs8"
)

// JWK represents a parsed JSON Web Key with all standard attributes (RFC 7517).
type JWK struct {
	KeyType    string   `json:"kty"`
	Use        string   `json:"use,omitempty"`
	KeyOps     []string `json:"key_ops,omitempty"`
	Algorithm  string   `json:"alg,omitempty"`
	KeyID      string   `json:"kid,omitempty"`
	PrivateKey *rsa.PrivateKey
}

// ParseJWK parses a JWK (JSON Web Key) and returns a JWK struct with all
// standard attributes and the extracted RSA private key.
func ParseJWK(jwkData []byte) (*JWK, error) {
	var raw struct {
		Kty    string   `json:"kty"`
		Use    string   `json:"use,omitempty"`
		KeyOps []string `json:"key_ops,omitempty"`
		Alg    string   `json:"alg,omitempty"`
		Kid    string   `json:"kid,omitempty"`
		N      string   `json:"n"`
		E      string   `json:"e"`
		D      string   `json:"d"`
		P      string   `json:"p"`
		Q      string   `json:"q"`
		Dp     string   `json:"dp"`
		Dq     string   `json:"dq"`
		Qi     string   `json:"qi"`
	}

	if err := json.Unmarshal(jwkData, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	if raw.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", raw.Kty)
	}

	decodeB64 := func(s string) (*big.Int, error) {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetBytes(b), nil
	}

	n, err := decodeB64(raw.N)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus (n): %w", err)
	}
	e, err := decodeB64(raw.E)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent (e): %w", err)
	}
	d, err := decodeB64(raw.D)
	if err != nil {
		return nil, fmt.Errorf("invalid private exponent (d): %w", err)
	}
	p, err := decodeB64(raw.P)
	if err != nil {
		return nil, fmt.Errorf("invalid prime (p): %w", err)
	}
	q, err := decodeB64(raw.Q)
	if err != nil {
		return nil, fmt.Errorf("invalid prime (q): %w", err)
	}
	dp, err := decodeB64(raw.Dp)
	if err != nil {
		return nil, fmt.Errorf("invalid dp: %w", err)
	}
	dq, err := decodeB64(raw.Dq)
	if err != nil {
		return nil, fmt.Errorf("invalid dq: %w", err)
	}
	qi, err := decodeB64(raw.Qi)
	if err != nil {
		return nil, fmt.Errorf("invalid qi: %w", err)
	}

	key := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: n,
			E: int(e.Int64()),
		},
		D:      d,
		Primes: []*big.Int{p, q},
	}
	key.Precomputed = rsa.PrecomputedValues{
		Dp:   dp,
		Dq:   dq,
		Qinv: qi,
	}

	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("invalid RSA key: %w", err)
	}

	return &JWK{
		KeyType:    raw.Kty,
		Use:        raw.Use,
		KeyOps:     raw.KeyOps,
		Algorithm:  raw.Alg,
		KeyID:      raw.Kid,
		PrivateKey: key,
	}, nil
}

// ParseRSAPrivateKey auto-detects JWK (JSON) vs PEM/PKCS8 format and parses
// the RSA private key. Optional passphrase for encrypted PKCS8 keys.
func ParseRSAPrivateKey(keyData []byte, passphrase ...[]byte) (*rsa.PrivateKey, error) {
	// Trim whitespace for format detection.
	t := bytes.TrimSpace(keyData)
	if len(t) == 0 {
		return nil, fmt.Errorf("empty key data")
	}

	// Detect JWK (JSON object).
	if t[0] == '{' {
		jwk, err := ParseJWK(t)
		if err != nil {
			return nil, err
		}
		return jwk.PrivateKey, nil
	}

	// PEM-encoded key.
	der, _ := pem.Decode(keyData)
	if der == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	var key *rsa.PrivateKey
	var err error

	if len(passphrase) > 0 && len(passphrase[0]) > 0 {
		key, err = pkcs8.ParsePKCS8PrivateKeyRSA(der.Bytes, passphrase[0])
	} else {
		key, err = pkcs8.ParsePKCS8PrivateKeyRSA(der.Bytes)
	}
	if err != nil {
		return nil, err
	}

	if err := key.Validate(); err != nil {
		return nil, err
	}

	return key, nil
}

// jwtEncode encodes input to base64 URL-encoded string with trailing '=' removed.
func jwtEncode(input interface{}) (string, error) {
	var data []byte

	switch i := input.(type) {
	case []byte:
		data = i
	default:
		var err error
		data, err = json.Marshal(input)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "="), nil
}

// JWT signing algorithm.
type JWTAlgorithm string

const (
	RS256 JWTAlgorithm = "RS256"
	RS512 JWTAlgorithm = "RS512"
)

// SignJWT creates a signed JWT token (header.payload.signature) using the
// specified algorithm (RS256 or RS512). Claims can be map[string]interface{}
// or a struct. Optional headerFields adds custom JWT header fields (e.g. "kid", "type").
func SignJWT(alg JWTAlgorithm, key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	var hash crypto.Hash
	switch alg {
	case RS256:
		hash = crypto.SHA256
	case RS512:
		hash = crypto.SHA512
	default:
		return "", fmt.Errorf("unsupported JWT algorithm: %s", alg)
	}

	hdr := map[string]string{"alg": string(alg)}
	for _, fields := range headerFields {
		for k, v := range fields {
			hdr[k] = v
		}
	}

	header, err := jwtEncode(hdr)
	if err != nil {
		return "", err
	}

	payload, err := jwtEncode(claims)
	if err != nil {
		return "", err
	}

	h := hash.New()
	h.Write([]byte(fmt.Sprintf("%s.%s", header, payload)))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, hash, h.Sum(nil))
	if err != nil {
		return "", err
	}

	sigStr, err := jwtEncode(sig)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s.%s", header, payload, sigStr), nil
}

// SignRS256 creates a signed JWT token using RS256 (RSA SHA-256).
func SignRS256(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	return SignJWT(RS256, key, claims, headerFields...)
}

// SignRS512 creates a signed JWT token using RS512 (RSA SHA-512).
func SignRS512(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	return SignJWT(RS512, key, claims, headerFields...)
}
