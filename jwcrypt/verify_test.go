package jwcrypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"
)

// One key for the whole package: RSA generation dominates the runtime of these
// tests otherwise, and none of them care which key they got.
var (
	testKeyOnce sync.Once
	testKey     *rsa.PrivateKey
)

func key(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testKeyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		testKey = k
	})
	return testKey
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func publicJWK(t *testing.T, k *rsa.PrivateKey, kid string, extra string) []byte {
	t.Helper()
	e := big.NewInt(int64(k.E))
	doc := fmt.Sprintf(`{"kty":"RSA","use":"sig","alg":"RS256","kid":%q,"n":%q,"e":%q%s}`,
		kid, b64(k.N.Bytes()), b64(e.Bytes()), extra)
	return []byte(doc)
}

func privateJWK(t *testing.T, k *rsa.PrivateKey, kid string) []byte {
	t.Helper()
	e := big.NewInt(int64(k.E))
	doc := fmt.Sprintf(`{"kty":"RSA","kid":%q,"n":%q,"e":%q,"d":%q,"p":%q,"q":%q}`,
		kid, b64(k.N.Bytes()), b64(e.Bytes()),
		b64(k.D.Bytes()), b64(k.Primes[0].Bytes()), b64(k.Primes[1].Bytes()))
	return []byte(doc)
}

// validClaims is a token payload that passes every check, so a test can change
// exactly the one thing it is about.
func validClaims(extra map[string]interface{}) map[string]interface{} {
	c := map[string]interface{}{
		"iss": "https://api.botframework.com",
		"aud": "app-id-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
	}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func strictOpts() VerifyOptions {
	return VerifyOptions{Issuer: "https://api.botframework.com", Audience: "app-id-123"}
}

// --- round trip --------------------------------------------------------------

func TestSignVerifyRoundTrip(t *testing.T) {
	k := key(t)
	for _, alg := range []JWTAlgorithm{RS256, RS512} {
		token, err := SignJWT(alg, k, validClaims(map[string]interface{}{"sub": "u1"}),
			map[string]string{"kid": "k1"})
		if err != nil {
			t.Fatalf("%s sign: %v", alg, err)
		}
		claims, err := VerifyJWT(token, &k.PublicKey, strictOpts())
		if err != nil {
			t.Fatalf("%s verify: %v", alg, err)
		}
		if claims.String("sub") != "u1" {
			t.Errorf("%s: sub = %q, want u1", alg, claims.String("sub"))
		}
	}
}

func TestDecodeJWTHeaderReadsKid(t *testing.T) {
	k := key(t)
	token, err := SignRS256(k, validClaims(nil), map[string]string{"kid": "rotate-me"})
	if err != nil {
		t.Fatal(err)
	}
	hdr, err := DecodeJWTHeader(token)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.KeyID != "rotate-me" || hdr.Algorithm != RS256 {
		t.Errorf("header = %+v, want kid=rotate-me alg=RS256", hdr)
	}
}

// --- signature integrity -----------------------------------------------------

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	k := key(t)
	token, err := SignRS256(k, validClaims(map[string]interface{}{"sub": "u1"}))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	forged, err := jwtEncode(validClaims(map[string]interface{}{"sub": "admin"}))
	if err != nil {
		t.Fatal(err)
	}
	tampered := parts[0] + "." + forged + "." + parts[2]
	if _, err := VerifyJWT(tampered, &k.PublicKey, strictOpts()); err == nil {
		t.Fatal("a re-written payload verified")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	k := key(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	token, err := SignRS256(k, validClaims(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJWT(token, &other.PublicKey, strictOpts()); err == nil {
		t.Fatal("token verified against an unrelated key")
	}
}

// TestVerifyRejectsAlgorithmConfusion covers the attack the allow-list exists
// for: a token whose header talks the verifier out of doing RSA.
func TestVerifyRejectsAlgorithmConfusion(t *testing.T) {
	k := key(t)
	token, err := SignRS256(k, validClaims(nil))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")

	for _, alg := range []string{"none", "HS256", "RS384"} {
		hdr, err := jwtEncode(map[string]string{"alg": alg})
		if err != nil {
			t.Fatal(err)
		}
		forged := hdr + "." + parts[1] + "." + parts[2]
		if _, err := VerifyJWT(forged, &k.PublicKey, strictOpts()); err == nil {
			t.Errorf("alg=%q was accepted", alg)
		}
	}

	// "none" with the signature stripped entirely, the textbook form.
	hdr, _ := jwtEncode(map[string]string{"alg": "none"})
	if _, err := VerifyJWT(hdr+"."+parts[1]+".", &k.PublicKey, strictOpts()); err == nil {
		t.Error("alg=none with an empty signature was accepted")
	}
}

// A token really signed RS256 must not verify when its header claims RS512:
// the header selects the hash, so a swap has to fail on the signature.
func TestVerifyHashSwapFails(t *testing.T) {
	k := key(t)
	token, err := SignRS256(k, validClaims(nil))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	hdr, _ := jwtEncode(map[string]string{"alg": "RS512"})
	if _, err := VerifyJWT(hdr+"."+parts[1]+"."+parts[2], &k.PublicKey, strictOpts()); err == nil {
		t.Fatal("RS256 signature verified as RS512")
	}
}

func TestVerifyMalformedToken(t *testing.T) {
	k := key(t)
	for _, tok := range []string{"", "a.b", "a.b.c.d", "!!!.!!!.!!!"} {
		if _, err := VerifyJWT(tok, &k.PublicKey, strictOpts()); err == nil {
			t.Errorf("malformed token %q verified", tok)
		}
	}
	if _, err := VerifyJWT("a.b.c", nil, strictOpts()); err == nil {
		t.Error("a nil key verified something")
	}
}

// --- claim checks ------------------------------------------------------------

func TestVerifyClaimChecks(t *testing.T) {
	k := key(t)
	sign := func(c map[string]interface{}) string {
		tok, err := SignRS256(k, c)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	tests := []struct {
		name    string
		claims  map[string]interface{}
		opts    VerifyOptions
		wantErr string
	}{
		{"issuer mismatch",
			validClaims(map[string]interface{}{"iss": "https://evil.example"}),
			strictOpts(), "issuer mismatch"},
		{"audience mismatch",
			validClaims(map[string]interface{}{"aud": "someone-else"}),
			strictOpts(), "audience mismatch"},
		{"audience array matches",
			validClaims(map[string]interface{}{"aud": []string{"other", "app-id-123"}}),
			strictOpts(), ""},
		{"audience array misses",
			validClaims(map[string]interface{}{"aud": []string{"other", "third"}}),
			strictOpts(), "audience mismatch"},
		{"expired",
			validClaims(map[string]interface{}{"exp": time.Now().Add(-time.Hour).Unix()}),
			strictOpts(), "expired"},
		{"expired within leeway",
			validClaims(map[string]interface{}{"exp": time.Now().Add(-30 * time.Second).Unix()}),
			VerifyOptions{Issuer: "https://api.botframework.com", Audience: "app-id-123", Leeway: 2 * time.Minute}, ""},
		{"not yet valid",
			validClaims(map[string]interface{}{"nbf": time.Now().Add(time.Hour).Unix()}),
			strictOpts(), "not valid until"},
		{"missing exp refused by default",
			map[string]interface{}{"iss": "https://api.botframework.com", "aud": "app-id-123"},
			strictOpts(), "no exp claim"},
		{"missing exp allowed on request",
			map[string]interface{}{"iss": "https://api.botframework.com", "aud": "app-id-123"},
			VerifyOptions{Issuer: "https://api.botframework.com", Audience: "app-id-123", AllowMissingExpiry: true}, ""},
		{"unchecked issuer and audience",
			validClaims(map[string]interface{}{"iss": "anyone", "aud": "anything"}),
			VerifyOptions{}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyJWT(sign(tc.claims), &k.PublicKey, tc.opts)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestVerifyUsesInjectedClock(t *testing.T) {
	k := key(t)
	// Issued for an hour, checked a day later.
	token, err := SignRS256(k, validClaims(nil))
	if err != nil {
		t.Fatal(err)
	}
	opts := strictOpts()
	opts.Now = func() time.Time { return time.Now().Add(24 * time.Hour) }
	if _, err := VerifyJWT(token, &k.PublicKey, opts); err == nil {
		t.Fatal("a day-old token passed")
	}
}

func TestClaimsAccessors(t *testing.T) {
	var c Claims
	if err := json.Unmarshal([]byte(`{"s":"x","arr":["a","b"],"n":1700000000,"mixed":["a",2]}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.String("s") != "x" || c.String("missing") != "" || c.String("n") != "" {
		t.Error("String accessor wrong")
	}
	if got := c.Strings("s"); len(got) != 1 || got[0] != "x" {
		t.Errorf("Strings on a scalar = %v, want [x]", got)
	}
	if got := c.Strings("arr"); len(got) != 2 || got[1] != "b" {
		t.Errorf("Strings on an array = %v", got)
	}
	if got := c.Strings("mixed"); len(got) != 1 || got[0] != "a" {
		t.Errorf("Strings skipping non-strings = %v, want [a]", got)
	}
	if ts, ok := c.Time("n"); !ok || ts.Unix() != 1700000000 {
		t.Errorf("Time = %v %v", ts, ok)
	}
	if _, ok := c.Time("s"); ok {
		t.Error("Time accepted a string claim")
	}
}

// --- JWK parsing -------------------------------------------------------------

// The bug this whole file was blocked on: a public-only JWK, which is what every
// issuer actually publishes, used to fail to parse at all.
func TestParseJWKPublicOnly(t *testing.T) {
	k := key(t)
	jwk, err := ParseJWK(publicJWK(t, k, "k1", ""))
	if err != nil {
		t.Fatalf("public JWK did not parse: %v", err)
	}
	if jwk.PrivateKey != nil {
		t.Error("a public-only JWK produced a private key")
	}
	if jwk.PublicKey == nil || jwk.PublicKey.N.Cmp(k.N) != 0 || jwk.PublicKey.E != k.E {
		t.Fatal("public key did not round-trip")
	}
	if jwk.KeyID != "k1" || jwk.Use != "sig" || jwk.Algorithm != "RS256" {
		t.Errorf("attributes lost: %+v", jwk)
	}
}

func TestParseJWKPrivateStillWorks(t *testing.T) {
	k := key(t)
	jwk, err := ParseJWK(privateJWK(t, k, "k1"))
	if err != nil {
		t.Fatalf("private JWK: %v", err)
	}
	if jwk.PrivateKey == nil {
		t.Fatal("no private key")
	}
	if jwk.PrivateKey.D.Cmp(k.D) != 0 {
		t.Error("private exponent did not round-trip")
	}
	// Precompute must have run, or every signature with this key takes the slow
	// path silently.
	if jwk.PrivateKey.Precomputed.Dp == nil {
		t.Error("Precomputed values were not derived")
	}
	// And it must actually sign.
	tok, err := SignRS256(jwk.PrivateKey, validClaims(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJWT(tok, &k.PublicKey, strictOpts()); err != nil {
		t.Fatalf("key parsed from JWK produced an unverifiable signature: %v", err)
	}
}

func TestParseJWKIncompletePrivate(t *testing.T) {
	k := key(t)
	e := big.NewInt(int64(k.E))
	doc := fmt.Sprintf(`{"kty":"RSA","n":%q,"e":%q,"d":%q}`,
		b64(k.N.Bytes()), b64(e.Bytes()), b64(k.D.Bytes()))
	if _, err := ParseJWK([]byte(doc)); err == nil {
		t.Fatal("a JWK with d but no primes parsed")
	}
}

func TestParseJWKFromX5C(t *testing.T) {
	k := key(t)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"kty":"RSA","kid":"c1","x5c":[%q]}`, base64.StdEncoding.EncodeToString(der))
	jwk, err := ParseJWK([]byte(doc))
	if err != nil {
		t.Fatalf("x5c-only JWK: %v", err)
	}
	if jwk.PublicKey == nil || jwk.PublicKey.N.Cmp(k.N) != 0 {
		t.Fatal("public key not recovered from x5c")
	}
}

func TestParseJWKKeepsExtraMembers(t *testing.T) {
	k := key(t)
	jwk, err := ParseJWK(publicJWK(t, k, "k1", `,"endorsements":["msteams"]`))
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := jwk.Extra["endorsements"]
	if !ok {
		t.Fatal("endorsements not preserved")
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil || len(got) != 1 || got[0] != "msteams" {
		t.Errorf("endorsements = %v (%v)", got, err)
	}
	if _, leaked := jwk.Extra["n"]; leaked {
		t.Error("a standard member leaked into Extra")
	}
}

func TestParseJWKRejectsBadInput(t *testing.T) {
	for _, doc := range []string{
		`{"kty":"EC","crv":"P-256"}`,
		`{"kty":"RSA"}`,
		`{"kty":"RSA","n":"!!!","e":"AQAB"}`,
		`not json`,
	} {
		if _, err := ParseJWK([]byte(doc)); err == nil {
			t.Errorf("%s parsed", doc)
		}
	}
}

// ParseRSAPrivateKey used to lean on ParseJWK failing for public keys. Now that
// it succeeds, the nil private half has to be caught here or the caller gets a
// nil key and a nil error.
func TestParseRSAPrivateKeyRejectsPublicJWK(t *testing.T) {
	k := key(t)
	got, err := ParseRSAPrivateKey(publicJWK(t, k, "k1", ""))
	if err == nil {
		t.Fatal("a public JWK was accepted as a private key")
	}
	if got != nil {
		t.Error("a key was returned alongside the error")
	}
}

func TestParseRSAPrivateKeyFromJWK(t *testing.T) {
	k := key(t)
	got, err := ParseRSAPrivateKey(privateJWK(t, k, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.D.Cmp(k.D) != 0 {
		t.Error("private key did not round-trip")
	}
}

// --- JWK sets ----------------------------------------------------------------

func TestParseJWKSetAndVerify(t *testing.T) {
	k := key(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"keys":[%s,%s]}`,
		publicJWK(t, other, "other", ""), publicJWK(t, k, "k1", ""))

	set, err := ParseJWKSet([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("parsed %d keys, want 2", len(set.Keys))
	}
	if _, ok := set.KeyByID("k1"); !ok {
		t.Fatal("KeyByID missed k1")
	}
	if _, ok := set.KeyByID("nope"); ok {
		t.Fatal("KeyByID invented a key")
	}

	token, err := SignRS256(k, validClaims(nil), map[string]string{"kid": "k1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.VerifyJWT(token, strictOpts()); err != nil {
		t.Fatalf("set verify: %v", err)
	}

	// Signed by k but naming the other key: the set must not fall back to
	// trying everything when a kid is present.
	mislabelled, err := SignRS256(k, validClaims(nil), map[string]string{"kid": "other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.VerifyJWT(mislabelled, strictOpts()); err == nil {
		t.Fatal("a token naming the wrong kid verified")
	}
}

// An unknown kid means the issuer rotated, and the caller re-fetches. A bad
// signature means reject. The error type is how they are told apart.
func TestUnknownKeyIDIsDistinguishable(t *testing.T) {
	k := key(t)
	set, err := ParseJWKSet([]byte(fmt.Sprintf(`{"keys":[%s]}`, publicJWK(t, k, "k1", ""))))
	if err != nil {
		t.Fatal(err)
	}
	token, err := SignRS256(k, validClaims(nil), map[string]string{"kid": "rotated-in"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = set.VerifyJWT(token, strictOpts())
	var unknown *UnknownKeyIDError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %v (%T), want *UnknownKeyIDError", err, err)
	}
	if unknown.KeyID != "rotated-in" {
		t.Errorf("KeyID = %q", unknown.KeyID)
	}
}

func TestJWKSetSkipsUnusableKeys(t *testing.T) {
	k := key(t)
	doc := fmt.Sprintf(`{"keys":[{"kty":"EC","crv":"P-256","kid":"ec"},%s]}`, publicJWK(t, k, "k1", ""))
	set, err := ParseJWKSet([]byte(doc))
	if err != nil {
		t.Fatalf("a set with one unusable key failed entirely: %v", err)
	}
	if len(set.Keys) != 1 || set.Keys[0].KeyID != "k1" {
		t.Fatalf("keys = %+v", set.Keys)
	}

	if _, err := ParseJWKSet([]byte(`{"keys":[{"kty":"EC","crv":"P-256"}]}`)); err == nil {
		t.Error("a set with no usable key parsed")
	}
	if _, err := ParseJWKSet([]byte(`{"keys":[]}`)); err == nil {
		t.Error("an empty set parsed")
	}
	if _, err := ParseJWKSet([]byte(`nope`)); err == nil {
		t.Error("a non-JSON set parsed")
	}
}

func TestJWKSetNoKidTriesEveryKey(t *testing.T) {
	k := key(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	set, err := ParseJWKSet([]byte(fmt.Sprintf(`{"keys":[%s,%s]}`,
		publicJWK(t, other, "other", ""), publicJWK(t, k, "k1", ""))))
	if err != nil {
		t.Fatal(err)
	}
	token, err := SignRS256(k, validClaims(nil)) // no kid header
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.VerifyJWT(token, strictOpts()); err != nil {
		t.Fatalf("kid-less token: %v", err)
	}
}
