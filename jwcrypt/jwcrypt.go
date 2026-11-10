// Package jwcrypt signs and verifies RSA-based JSON Web Tokens, and parses the
// keys they are signed with: JWK documents (public or private), JWK Sets, and
// PEM/PKCS8 private keys.
//
// It does no network I/O. Fetching an issuer's OIDC discovery document and
// caching the keys behind it are application decisions -- which HTTP client,
// which timeouts, when a rotation warrants a re-fetch -- so a caller retrieves
// the JWK Set itself and hands the bytes to ParseJWKSet.
package jwcrypt

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"

	"github.com/youmark/pkcs8"
)

// JWK represents a parsed JSON Web Key with all standard attributes (RFC 7517).
//
// A JWK may carry a public key, a private key, or both. PublicKey is populated
// whenever the key parses at all; PrivateKey only when the private parameters
// are present. An issuer's published signing keys -- the JWK Set behind an OIDC
// discovery document -- are public-only, so a caller verifying a token reads
// PublicKey and must expect PrivateKey to be nil.
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

// jwkStandardMembers are the members ParseJWK decodes itself; anything else in
// the document lands in JWK.Extra.
var jwkStandardMembers = map[string]bool{
	"kty": true, "use": true, "key_ops": true, "alg": true, "kid": true,
	"x5c": true, "x5t": true, "x5u": true, "x5t#S256": true,
	"n": true, "e": true, "d": true, "p": true, "q": true,
	"dp": true, "dq": true, "qi": true, "oth": true,
}

// ParseJWK parses a JWK (JSON Web Key) and returns a JWK struct with all
// standard attributes and the extracted RSA key material.
//
// Public-only keys are supported: when the private parameters are absent the
// result carries PublicKey and a nil PrivateKey. That is the normal shape of a
// key fetched from an issuer's JWK Set, and rejecting it -- as this function
// once did, by unconditionally assembling and validating an rsa.PrivateKey --
// left no way to verify anything this package signed.
func ParseJWK(jwkData []byte) (*JWK, error) {
	var raw struct {
		Kty    string   `json:"kty"`
		Use    string   `json:"use,omitempty"`
		KeyOps []string `json:"key_ops,omitempty"`
		Alg    string   `json:"alg,omitempty"`
		Kid    string   `json:"kid,omitempty"`
		X5C    []string `json:"x5c,omitempty"`
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

	out := &JWK{
		KeyType:   raw.Kty,
		Use:       raw.Use,
		KeyOps:    raw.KeyOps,
		Algorithm: raw.Alg,
		KeyID:     raw.Kid,
		X5C:       raw.X5C,
	}

	var members map[string]json.RawMessage
	if json.Unmarshal(jwkData, &members) == nil {
		for name, value := range members {
			if jwkStandardMembers[name] {
				continue
			}
			if out.Extra == nil {
				out.Extra = make(map[string]json.RawMessage)
			}
			out.Extra[name] = value
		}
	}

	pub, err := jwkPublicKey(raw.N, raw.E, raw.X5C)
	if err != nil {
		return nil, err
	}
	out.PublicKey = pub

	// Private parameters are optional. Absent them this is a public key and we
	// are done.
	if raw.D == "" && raw.P == "" && raw.Q == "" {
		return out, nil
	}
	if raw.D == "" || raw.P == "" || raw.Q == "" {
		return nil, fmt.Errorf("incomplete private key: d, p and q are all required")
	}

	d, err := decodeJWKInt(raw.D)
	if err != nil {
		return nil, fmt.Errorf("invalid private exponent (d): %w", err)
	}
	p, err := decodeJWKInt(raw.P)
	if err != nil {
		return nil, fmt.Errorf("invalid prime (p): %w", err)
	}
	q, err := decodeJWKInt(raw.Q)
	if err != nil {
		return nil, fmt.Errorf("invalid prime (q): %w", err)
	}

	key := &rsa.PrivateKey{
		PublicKey: *pub,
		D:         d,
		Primes:    []*big.Int{p, q},
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("invalid RSA key: %w", err)
	}
	// Derive dp/dq/qi rather than trusting the document's. They are a function
	// of d, p and q, which Validate has just checked, and Precompute also fills
	// the unexported state the crypto/rsa fast paths look for -- which assigning
	// rsa.PrecomputedValues by hand does not, so a hand-filled key silently took
	// the slow path.
	key.Precompute()
	out.PrivateKey = key

	return out, nil
}

// jwkPublicKey builds the public key from the JWK's modulus and exponent,
// falling back to the leaf certificate in x5c for issuers that publish a chain
// and nothing else.
func jwkPublicKey(nStr, eStr string, x5c []string) (*rsa.PublicKey, error) {
	if nStr != "" && eStr != "" {
		n, err := decodeJWKInt(nStr)
		if err != nil {
			return nil, fmt.Errorf("invalid modulus (n): %w", err)
		}
		e, err := decodeJWKInt(eStr)
		if err != nil {
			return nil, fmt.Errorf("invalid exponent (e): %w", err)
		}
		// The exponent lands in an int, so a document claiming a huge one must be
		// refused rather than silently truncated.
		if e.Sign() <= 0 || e.BitLen() > 31 {
			return nil, fmt.Errorf("invalid exponent (e): out of range")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	}

	if len(x5c) > 0 {
		// x5c is standard base64 (RFC 7517 4.7), not the base64url the key
		// parameters use.
		der, err := base64.StdEncoding.DecodeString(x5c[0])
		if err != nil {
			return nil, fmt.Errorf("invalid x5c certificate: %w", err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("invalid x5c certificate: %w", err)
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("x5c certificate does not carry an RSA public key")
		}
		return pub, nil
	}

	return nil, fmt.Errorf("JWK carries neither n/e nor x5c")
}

// decodeJWKInt decodes a base64url JWK integer parameter.
func decodeJWKInt(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
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
		// Public JWKs parse successfully now, so an absent private half is no
		// longer an error ParseJWK reports -- it has to be caught here, or this
		// returns a nil key alongside a nil error.
		if jwk.PrivateKey == nil {
			return nil, fmt.Errorf("JWK carries only a public key")
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
	// PS256 is RSA-PSS with SHA-256, the salt as long as the hash (RFC 7518
	// 3.5). Microsoft Entra asks for it on a certificate's client assertion.
	PS256 JWTAlgorithm = "PS256"
)

// pssOptions are PS256's: a salt as long as the hash, as RFC 7518 requires.
var pssOptions = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}

// SignJWT creates a signed JWT token (header.payload.signature) using the
// specified algorithm (RS256, RS512 or PS256). Claims can be map[string]interface{}
// or a struct. Optional headerFields adds custom JWT header fields (e.g. "kid", "type").
func SignJWT(alg JWTAlgorithm, key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	var hash crypto.Hash
	switch alg {
	case RS256:
		hash = crypto.SHA256
	case RS512:
		hash = crypto.SHA512
	case PS256:
		hash = crypto.SHA256
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
	var sig []byte
	if alg == PS256 {
		sig, err = rsa.SignPSS(rand.Reader, key, hash, h.Sum(nil), pssOptions)
	} else {
		sig, err = rsa.SignPKCS1v15(rand.Reader, key, hash, h.Sum(nil))
	}
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

// SignPS256 creates a signed JWT token using PS256 (RSA-PSS SHA-256).
func SignPS256(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	return SignJWT(PS256, key, claims, headerFields...)
}

// SignRS512 creates a signed JWT token using RS512 (RSA SHA-512).
func SignRS512(key *rsa.PrivateKey, claims interface{}, headerFields ...map[string]string) (string, error) {
	return SignJWT(RS512, key, claims, headerFields...)
}
