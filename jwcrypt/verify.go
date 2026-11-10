// JWT verification: the other half of SignJWT.
//
// A library that signs but cannot verify is half a library, and the gap was
// load-bearing: ParseJWK assembled an rsa.PrivateKey unconditionally, so it
// rejected every public-only key an issuer actually publishes, which made the
// verifying direction impossible to write against a real JWK Set.
//
// Deliberately no network. Fetching a discovery document, caching keys, and
// deciding when a rotation warrants a re-fetch are policy: they belong to the
// application, which knows its own HTTP client, timeouts, and logging. This
// file does the cryptography and the claim checks, and nothing else. The one
// concession to that boundary is UnknownKeyIDError, which exists so a caller
// can tell "the issuer rotated, re-fetch" apart from "this token is bad".

package jwcrypt

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// defaultVerifyAlgorithms are the algorithms accepted when VerifyOptions names
// none. Both are asymmetric: an allow-list that admits an HMAC algorithm would
// let a token's own header talk a verifier into treating a public key as a
// shared secret.
var defaultVerifyAlgorithms = []JWTAlgorithm{RS256, RS512}

// Claims is a decoded JWT payload. The accessors handle the shapes RFC 7519
// actually permits, which is where hand-rolled claim checks usually go wrong.
type Claims map[string]interface{}

// String returns a string claim, or "" if absent or another type.
func (c Claims) String(name string) string {
	s, _ := c[name].(string)
	return s
}

// Strings returns a claim that RFC 7519 allows to be either a single string or
// an array of them (aud is the one that bites). A plain string yields a
// one-element slice; anything else yields nil.
func (c Claims) Strings(name string) []string {
	switch v := c[name].(type) {
	case string:
		return []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Time returns a NumericDate claim (exp, nbf, iat) and whether it was present
// and well-formed. JSON numbers decode as float64, so seconds are read that way
// and fractional seconds are kept.
func (c Claims) Time(name string) (time.Time, bool) {
	var secs float64
	switch v := c[name].(type) {
	case float64:
		secs = v
	case int64:
		secs = float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return time.Time{}, false
		}
		secs = f
	default:
		return time.Time{}, false
	}
	whole, frac := int64(secs), secs-float64(int64(secs))
	return time.Unix(whole, int64(frac*float64(time.Second))), true
}

// JWTHeader is a decoded JOSE header.
type JWTHeader struct {
	Algorithm JWTAlgorithm `json:"alg"`
	KeyID     string       `json:"kid,omitempty"`
	Type      string       `json:"typ,omitempty"`
}

// DecodeJWTHeader reads a token's header WITHOUT verifying anything. It exists
// so a caller can learn which key the token names (kid) before it has a key to
// check it with. Nothing it returns is trustworthy until VerifyJWT succeeds.
func DecodeJWTHeader(token string) (JWTHeader, error) {
	var hdr JWTHeader
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return hdr, fmt.Errorf("malformed JWT: expected 3 dot-separated segments, got %d", len(parts))
	}
	raw, err := jwtDecode(parts[0])
	if err != nil {
		return hdr, fmt.Errorf("malformed JWT header: %w", err)
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return hdr, fmt.Errorf("malformed JWT header: %w", err)
	}
	return hdr, nil
}

// VerifyOptions controls what VerifyJWT insists on beyond the signature.
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

func (o VerifyOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// VerifyJWT checks a token's signature against key and then its standard
// claims, returning the decoded payload. The token's own "alg" header selects a
// hash only from the caller's allow-list, and never selects the key type: that
// is the whole defence against algorithm-confusion, where a token declaring
// HS256 (or "none") talks a verifier out of doing RSA at all.
func VerifyJWT(token string, key *rsa.PublicKey, opts VerifyOptions) (Claims, error) {
	if key == nil {
		return nil, fmt.Errorf("no public key supplied")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 dot-separated segments, got %d", len(parts))
	}

	hdr, err := DecodeJWTHeader(token)
	if err != nil {
		return nil, err
	}
	hash, err := verifyHash(hdr.Algorithm, opts.Algorithms)
	if err != nil {
		return nil, err
	}

	sig, err := jwtDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("malformed JWT signature: %w", err)
	}
	h := hash.New()
	h.Write([]byte(parts[0] + "." + parts[1]))
	if hdr.Algorithm == PS256 {
		err = rsa.VerifyPSS(key, hash, h.Sum(nil), sig, pssOptions)
	} else {
		err = rsa.VerifyPKCS1v15(key, hash, h.Sum(nil), sig)
	}
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	payload, err := jwtDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed JWT payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("malformed JWT payload: %w", err)
	}
	if err := opts.checkClaims(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// checkClaims applies the registered-claim checks. Signature first, claims
// second: there is no point reasoning about the contents of a token nobody
// signed.
func (o VerifyOptions) checkClaims(c Claims) error {
	now := o.now()

	if o.Issuer != "" {
		if got := c.String("iss"); got != o.Issuer {
			return fmt.Errorf("issuer mismatch: token says %q, want %q", got, o.Issuer)
		}
	}
	if o.Audience != "" {
		matched := false
		for _, aud := range c.Strings("aud") {
			if aud == o.Audience {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("audience mismatch: token is for %v, want %q", c.Strings("aud"), o.Audience)
		}
	}

	if exp, ok := c.Time("exp"); ok {
		if now.After(exp.Add(o.Leeway)) {
			return fmt.Errorf("token expired at %s", exp.UTC().Format(time.RFC3339))
		}
	} else if !o.AllowMissingExpiry {
		return fmt.Errorf("token has no exp claim (set AllowMissingExpiry to accept one)")
	}

	if nbf, ok := c.Time("nbf"); ok && now.Add(o.Leeway).Before(nbf) {
		return fmt.Errorf("token is not valid until %s", nbf.UTC().Format(time.RFC3339))
	}
	return nil
}

// verifyHash maps the token's declared algorithm to a hash, but only after the
// allow-list has admitted it.
func verifyHash(alg JWTAlgorithm, allowed []JWTAlgorithm) (crypto.Hash, error) {
	if len(allowed) == 0 {
		allowed = defaultVerifyAlgorithms
	}
	permitted := false
	for _, a := range allowed {
		if a == alg {
			permitted = true
			break
		}
	}
	if !permitted {
		names := make([]string, 0, len(allowed))
		for _, a := range allowed {
			names = append(names, string(a))
		}
		return 0, fmt.Errorf("token algorithm %q is not accepted (allowed: %s)", alg, strings.Join(names, ", "))
	}
	switch alg {
	case RS256:
		return crypto.SHA256, nil
	case RS512:
		return crypto.SHA512, nil
	case PS256:
		return crypto.SHA256, nil
	}
	return 0, fmt.Errorf("unsupported JWT algorithm: %s", alg)
}

// jwtDecode decodes a base64url JWT segment. SignJWT strips padding, and most
// issuers do too, but padded segments are accepted rather than rejected on a
// technicality.
func jwtDecode(seg string) ([]byte, error) {
	if pad := len(seg) % 4; pad != 0 {
		seg += strings.Repeat("=", 4-pad)
	}
	return base64.URLEncoding.DecodeString(seg)
}

// --- JWK Sets ----------------------------------------------------------------

// JWKSet is a parsed RFC 7517 key set, the document an OIDC issuer publishes at
// its jwks_uri.
type JWKSet struct {
	Keys []*JWK
}

// UnknownKeyIDError reports that a token names a key the set does not hold.
// Distinct from a verification failure on purpose: an unknown kid usually means
// the issuer rotated its keys and the caller should re-fetch the set, whereas a
// failed signature means reject the token and move on. Conflating the two gives
// you either a bridge that dies at the next rotation or one that re-fetches on
// every forged token.
type UnknownKeyIDError struct{ KeyID string }

func (e *UnknownKeyIDError) Error() string {
	return fmt.Sprintf("no key with kid %q in set", e.KeyID)
}

// ParseJWKSet parses a {"keys":[...]} document. Keys this package cannot use
// (EC, OKP, malformed entries) are skipped rather than failing the document:
// issuers mix key types freely, and one unusable entry should not take the
// usable ones with it. A set with no usable key at all is an error.
func ParseJWKSet(data []byte) (*JWKSet, error) {
	var doc struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JWK Set: %w", err)
	}
	if len(doc.Keys) == 0 {
		return nil, fmt.Errorf("JWK Set contains no keys")
	}

	set := &JWKSet{}
	var skipped int
	for _, raw := range doc.Keys {
		key, err := ParseJWK(raw)
		if err != nil || key.PublicKey == nil {
			skipped++
			continue
		}
		set.Keys = append(set.Keys, key)
	}
	if len(set.Keys) == 0 {
		return nil, fmt.Errorf("JWK Set contains no usable RSA keys (%d entr(ies) skipped)", skipped)
	}
	return set, nil
}

// KeyByID returns the key with the given kid.
func (s *JWKSet) KeyByID(kid string) (*JWK, bool) {
	for _, key := range s.Keys {
		if key.KeyID == kid {
			return key, true
		}
	}
	return nil, false
}

// VerifyJWT selects the key the token's kid names and verifies against it,
// returning UnknownKeyIDError when the set does not hold that key. A token with
// no kid is tried against every key in the set, which some small issuers
// require; the first key's error is reported when none of them work, since a
// claim failure is the same for all of them.
func (s *JWKSet) VerifyJWT(token string, opts VerifyOptions) (Claims, error) {
	hdr, err := DecodeJWTHeader(token)
	if err != nil {
		return nil, err
	}
	if hdr.KeyID != "" {
		key, ok := s.KeyByID(hdr.KeyID)
		if !ok {
			return nil, &UnknownKeyIDError{KeyID: hdr.KeyID}
		}
		return VerifyJWT(token, key.PublicKey, opts)
	}

	var firstErr error
	for _, key := range s.Keys {
		claims, err := VerifyJWT(token, key.PublicKey, opts)
		if err == nil {
			return claims, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}
