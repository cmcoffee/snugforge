package kvlite

// Authenticated encryption for stored values: AES-256-GCM.
//
// A value written by CryptSet has always been AES-CFB, under one IV for every
// value and with nothing to detect a change. GCM fixes both: a fresh random
// nonce per value, and a tag that fails the read when the value was altered.
// The value is also bound to the table and key it was written under (the
// additional data), so a value copied over a different key does not open.
//
// Stored form: flagGCM, then the 12-byte nonce, then the ciphertext and tag.
//
// This release READS GCM values and still writes CFB, so a database never
// holds a value an earlier release cannot read. A later release switches
// CryptSet to sealGCM and re-encrypts what is stored; by then every release
// someone might roll back to reads both.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// The first byte of a stored value says what follows it.
const (
	flagPlain byte = 0 // gob bytes as they are
	flagCFB   byte = 1 // gob bytes under AES-CFB (CryptSet until GCM)
	flagGCM   byte = 2 // gob bytes under AES-256-GCM (see above)
)

// ErrTampered is a GCM value that does not open: it was altered, copied from
// another table or key, or written under a different database key.
var ErrTampered = errors.New("kvlite: an encrypted value failed its integrity check")

// gcmKeyLabel separates the GCM key from the database key it is derived from,
// so the two ciphers never share key material.
const gcmKeyLabel = "kvlite aes-256-gcm v1\x00"

// gcm is the AEAD for this database key.
func (e encoder) gcm() (cipher.AEAD, error) {
	k := sha256.Sum256(append([]byte(gcmKeyLabel), e...))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// gcmAAD is what a value is bound to: the table (as stored, with any
// sub-store prefix) and the key.
func gcmAAD(table, key string) []byte {
	return []byte(table + "\x00" + key)
}

// sealGCM encrypts plain for table and key, in the stored form after the flag.
func (e encoder) sealGCM(table, key string, plain []byte) ([]byte, error) {
	aead, err := e.gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plain, gcmAAD(table, key)), nil
}

// openGCM is sealGCM's reverse: ErrTampered when the value does not open.
func (e encoder) openGCM(table, key string, sealed []byte) ([]byte, error) {
	aead, err := e.gcm()
	if err != nil {
		return nil, err
	}
	n := aead.NonceSize()
	if len(sealed) < n+aead.Overhead() {
		return nil, ErrTampered
	}
	plain, err := aead.Open(nil, sealed[:n], sealed[n:], gcmAAD(table, key))
	if err != nil {
		return nil, ErrTampered
	}
	return plain, nil
}

// isEncrypted reports whether a stored value is under either cipher.
func isEncrypted(v []byte) bool {
	return len(v) > 0 && (v[0] == flagCFB || v[0] == flagGCM)
}
