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
// CryptSet writes GCM. CFB values written before still read, and
// UpgradeCipher re-encrypts them. v0.3.5 was the release that only read GCM,
// so a rollback to it or later still reads everything.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"

	bolt "go.etcd.io/bbolt"
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

// upgradeChunk is how many values UpgradeCipher re-encrypts in one write
// transaction: small enough that a write waiting behind it is not held up
// long, large enough that a big store is not one commit per value.
const upgradeChunk = 500

// UpgradeProgress is how far an UpgradeCipher pass has got. Tables is known
// from the start, so a caller can say "140 of 312" while most tables turn out
// to hold nothing to upgrade.
type UpgradeProgress struct {
	Tables     int // tables in the store
	TablesDone int // tables scanned, and upgraded where they needed it
	Upgraded   int // values re-encrypted so far
}

// UpgradeCipher re-encrypts every AES-CFB value in s with AES-256-GCM, bound
// to its table and key. Plain and GCM values are left as they are, so it is
// safe to run on every open, and on a store being written meanwhile: each
// value is re-read and re-written inside the same write transaction, so a
// newer write is never overwritten. progress, when set, is told where the
// pass is after each chunk and after each table, whether or not that table
// held anything to upgrade. Returns how many values it re-encrypted.
//
// The CFB bytes stay in the file's freed pages until the file is compacted.
//
// Only a file-backed store holds anything to upgrade; any other returns 0.
func UpgradeCipher(s Store, progress func(UpgradeProgress)) (n int, err error) {
	K, ok := s.(*boltDB)
	if !ok {
		return 0, nil
	}
	tables, err := K.buckets(false)
	if err != nil {
		return 0, err
	}
	tell := func(done int) {
		if progress != nil {
			progress(UpgradeProgress{Tables: len(tables), TablesDone: done, Upgraded: n})
		}
	}
	for ti, table := range tables {
		var keys [][]byte
		err = K.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(table))
			if b == nil {
				return nil
			}
			return b.ForEach(func(k, v []byte) error {
				if len(v) > 0 && v[0] == flagCFB {
					keys = append(keys, append([]byte(nil), k...))
				}
				return nil
			})
		})
		if err != nil {
			return n, err
		}
		for start := 0; start < len(keys); start += upgradeChunk {
			end := start + upgradeChunk
			if end > len(keys) {
				end = len(keys)
			}
			done := 0
			err = K.db.Update(func(tx *bolt.Tx) error {
				done = 0
				b := tx.Bucket([]byte(table))
				if b == nil {
					return nil
				}
				for _, k := range keys[start:end] {
					v := b.Get(k)
					if len(v) == 0 || v[0] != flagCFB {
						continue // written since, or gone
					}
					sealed, err := K.encoder.sealGCM(table, string(k), K.encoder.decrypt(v[1:]))
					if err != nil {
						return err
					}
					if err := b.Put(k, append([]byte{flagGCM}, sealed...)); err != nil {
						return err
					}
					done++
				}
				return nil
			})
			if err != nil {
				return n, err
			}
			n += done
			if end < len(keys) {
				tell(ti) // mid-table: this one is not done yet
			}
		}
		tell(ti + 1)
	}
	return n, nil
}
