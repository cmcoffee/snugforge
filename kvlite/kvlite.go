package kvlite

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"strings"
	"time"
)

// ErrLocked indicates that the database is currently in use by another instance.
var ErrLocked = errors.New("Database is currently in use by an exisiting instance, please close it and try again.")

// Store provides a list of all tables.
// Table creats a key/val direct to a specified Table.
// Sub Creates a new bucket with a different namespace.
// Bucket Creates a new bucket for shared tenants.
// Drop drops the specified table.
// CountKeys provides a total of keys in table.
// Keys provides a listing of all keys in table.
// CryptSet encrypts the value within the key/value pair.
// Set sets the key/value pair in table.
// Unset deletes the key/value pair in table.
// Get retrieves value at key in table.
// Close closes the kvliter.Store.
// Buckets lists all bucket namespaces.
type Store interface {
	// Tables provides a list of all tables.
	Tables() (tables []string, err error)
	// AllTables lists EVERY bucket, including sub-store namespaces — the
	// separator-containing names Tables() hides. For maintenance / enumeration
	// (e.g. dropping all of a sub-scope's buckets by prefix).
	AllTables() (tables []string, err error)
	// Table creats a key/val direct to a specified Table.
	Table(table string) Table
	// SubStore Creates a new bucket with a different namespace, tied to
	Sub(name string) Store
	// SyncStore Creates a new bucket for shared tenants.
	Bucket(name string) Store
	// Drop drops the specified table.
	Drop(table string) (err error)
	// CountKeys provides a total of keys in table.
	CountKeys(table string) (count int, err error)
	// Keys provides a listing of all keys in table.
	Keys(table string) (keys []string, err error)
	// CryptSet encrypts the value within the key/value pair in table.
	CryptSet(table, key string, value interface{}) (err error)
	// Set sets the key/value pair in table.
	Set(table, key string, value interface{}) (err error)
	// Unset deletes the key/value pair in table.
	Unset(table, key string) (err error)
	// Get retrieves value at key in table.
	Get(table, key string, output interface{}) (found bool, err error)
	// ForEach reads every key in table in one read transaction, in key order
	// for a file store. decode decodes the key's value into out and may be
	// called zero or one times, so keys whose values are not needed skip the
	// decoding. A missing table calls fn zero times and returns nil; an error
	// from fn stops the read and is returned as is. fn must not write to the
	// store, as a write waits on the read fn is part of. Neither key nor the
	// decoded values point into the file once fn returns, and decode only
	// works while fn is running.
	ForEach(table string, fn func(key string, decode func(out interface{}) error) error) (err error)
	// Batch makes all the writes fn asks of b in one write transaction: one
	// commit, one sync. An error from fn writes nothing. The writes are
	// made after fn returns, so reads inside fn do not see them.
	Batch(fn func(b Batch) error) (err error)
	// Close closes the kvliter.Store.
	Close() (err error)
	// Buckets lists all bucket namespaces, limit_depth limits to first-level buckets
	buckets(limit_depth bool) (stores []string, err error)
}

// Table represents an interface for key-value storage.
// It provides methods for managing keys, values, and the table itself.
type Table interface {
	Keys() (keys []string, err error)
	CountKeys() (count int, err error)
	Set(key string, value interface{}) (err error)
	CryptSet(key string, value interface{}) (err error)
	Get(key string, value interface{}) (found bool, err error)
	Unset(key string) (err error)
	Drop() (err error)
	ForEach(fn func(key string, decode func(out interface{}) error) error) (err error)
}

// focused represents a focused view on a specific table within a store.
type focused struct {
	table string
	store Store
}

// Get retrieves the value associated with the given key from the table.
// Returns true if the key was found, false otherwise, and any error encountered.
func (s focused) Get(key string, value interface{}) (found bool, err error) {
	return s.store.Get(s.table, key, value)
}

// Keys returns a listing of all keys in the table.
func (s focused) Keys() (keys []string, err error) {
	return s.store.Keys(s.table)
}

// CountKeys returns the number of keys in the focused table.
func (s focused) CountKeys() (count int, err error) {
	return s.store.CountKeys(s.table)
}

// Set sets the key/value pair in the underlying store's table.
func (s focused) Set(key string, value interface{}) (err error) {
	return s.store.Set(s.table, key, value)
}

// CryptSet encrypts the value associated with the given key.
func (s focused) CryptSet(key string, value interface{}) (err error) {
	return s.store.CryptSet(s.table, key, value)
}

// Unset removes the key/value pair from the table.
func (s focused) Unset(key string) (err error) {
	return s.store.Unset(s.table, key)
}

// Drop removes the table from the store.
func (s focused) Drop() (err error) {
	return s.store.Drop(s.table)
}

// ForEach reads every key in the table in one read, see Store.ForEach.
func (s focused) ForEach(fn func(key string, decode func(out interface{}) error) error) (err error) {
	return s.store.ForEach(s.table, fn)
}

type boltDB struct {
	db      *bolt.DB
	encoder encoder
}

type encoder []byte

// buckets returns a list of bucket names.
// limit_depth controls whether to return full bucket paths or just the top-level names.
func (K *boltDB) buckets(limit_depth bool) (buckets []string, err error) {
	bmap := make(map[string]struct{})

	err = K.db.View(func(tx *bolt.Tx) error {
		add_bucket := func(name []byte, b *bolt.Bucket) error {
			name_str := string(name)
			if name_str == "KVLite" {
				return nil
			}
			if !limit_depth {
				buckets = append(buckets, name_str)
			} else {
				name_str = strings.Split(name_str, string(sepr))[0]
				if _, ok := bmap[name_str]; !ok {
					bmap[name_str] = struct{}{}
					buckets = append(buckets, name_str)
				}
			}
			return nil
		}
		return tx.ForEach(add_bucket)
	})
	return buckets, err
}

func hashBytes(input []byte) []byte {
	sum := sha256.Sum256(input)
	var output []byte
	output = append(output[0:], sum[0:]...)
	return output
}

// Encrypts bytes.
func (e encoder) encrypt(input []byte) []byte {

	key := hashBytes([]byte(e))
	block, _ := aes.NewCipher([]byte(e))

	buff := make([]byte, len(input))
	copy(buff, input)

	cipher.NewCFBEncrypter(block, key[0:block.BlockSize()]).XORKeyStream(buff, buff)

	return buff
}

// Decryps bytes.
func (e encoder) decrypt(input []byte) []byte {

	key := hashBytes([]byte(e))

	buff := make([]byte, len(input))
	copy(buff, input)

	block, _ := aes.NewCipher([]byte(e))
	cipher.NewCFBDecrypter(block, key[0:block.BlockSize()]).XORKeyStream(buff, buff)

	return buff
}

// Decodes input in to object.
func (e encoder) decode(input []byte, output interface{}) (err error) {
	var i []byte

	if input == nil {
		return nil
	}

	if input[0] == 1 {
		i = e.decrypt(input[1:])
	} else {
		i = input[1:]
	}

	x := gob.NewDecoder(bytes.NewBuffer(i))

	return x.Decode(output)
}

// Encodes input to bytes
func (e encoder) encode(input interface{}) (output []byte, err error) {
	buff := bytes.NewBuffer(nil)
	x := gob.NewEncoder(buff)
	err = x.Encode(input)
	return buff.Bytes(), err
}

// Encodes input to a stored value: a flag byte, 1 when encrypted and 0
// when not, then the gob bytes, encrypted when encrypt_value is set.
func (e encoder) pack(input interface{}, encrypt_value bool) (output []byte, err error) {
	v, err := e.encode(input)
	if err != nil {
		return nil, err
	}
	if encrypt_value {
		return append([]byte{1}, e.encrypt(v)...), nil
	}
	return append([]byte{0}, v...), nil
}

// Bucket returns the Store associated with the given bucket name.
func (K *boltDB) Bucket(name string) Store {
	return K.Sub(name)
}

// Sub returns a new substore with the given name.
func (K *boltDB) Sub(name string) Store {
	return &substore{fmt.Sprintf("%s%c", name, sepr), K}
}

// CountKeys returns the number of keys in the specified table.
// Returns 0 if the table does not exist.
func (K *boltDB) CountKeys(table string) (count int, err error) {
	err = K.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(table))
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	})
	return
}

// Keys returns a slice of strings representing all keys in the specified table.
// If the table does not exist, it returns an empty slice and no error.
func (K *boltDB) Keys(table string) (keys []string, err error) {
	err = K.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(table))
		if bucket == nil {
			return nil
		}
		add_key := func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		}
		return bucket.ForEach(add_key)
	})
	return keys, err
}

// Unset removes the key from the specified table. If the table
// or key does not exist, this operation does nothing.
func (K *boltDB) Unset(table, key string) (err error) {
	return K.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(table))
		if bucket == nil {
			return nil
		}
		if err = bucket.Delete([]byte(key)); err != nil {
			return err
		}
		return nil
	})
}

// Drop removes the specified table and all associated buckets, in one
// transaction: either all of them go or none do.
// If the table does not exist, this operation does nothing.
func (K *boltDB) Drop(table string) (err error) {
	sub_prefix := fmt.Sprintf("%s%c", table, sepr)
	return K.db.Update(func(tx *bolt.Tx) error {
		var tables [][]byte
		err := tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			name_str := string(name)
			if name_str == "KVLite" {
				return nil
			}
			if name_str == table || strings.HasPrefix(name_str, sub_prefix) {
				tables = append(tables, []byte(name_str))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, v := range tables {
			if err := tx.DeleteBucket(v); err != nil {
				return err
			}
		}
		return nil
	})
}

// Tables returns a list of table names.
// It retrieves all bucket names from the database that are not
// part of a sub-table (i.e., do not contain the separator rune).
func (K *boltDB) Tables() (tables []string, err error) {
	tmp, e := K.buckets(true)
	if e != nil {
		return tables, e
	}
	for _, v := range tmp {
		if !strings.ContainsRune(v, sepr) {
			tables = append(tables, v)
		}
	}
	return tables, err
}

// AllTables lists every bucket name verbatim, including the separator-containing
// sub-store namespaces Tables() filters out.
func (K *boltDB) AllTables() (tables []string, err error) {
	return K.buckets(false)
}

// Table returns a `Table` for the given table name.
func (K *boltDB) Table(table string) Table {
	return focused{table: table, store: K}
}

// Retrieves a value from the database.
// Returns whether the key was found and any error that occurred.
func (K *boltDB) Get(table, key string, output interface{}) (found bool, err error) {
	return found, K.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(table))
		if bucket == nil {
			found = false
			return nil
		}
		data := bucket.Get([]byte(key))
		if data != nil {
			found = true
			if output == nil {
				return nil
			}
		}
		return K.encoder.decode(data, output)
	})
}

// ForEach reads every key in table in one read transaction, see
// Store.ForEach.
func (K *boltDB) ForEach(table string, fn func(key string, decode func(out interface{}) error) error) (err error) {
	return K.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(table))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			return each_value(K.encoder, string(k), v, fn)
		})
	})
}

// Batch makes every write fn asks for in one write transaction, see
// Store.Batch. The values are encoded as fn asks, before the write lock is
// taken.
func (K *boltDB) Batch(fn func(b Batch) error) (err error) {
	staged := &staged_batch{encoder: K.encoder}
	if err = fn(staged); err != nil {
		return err
	}
	if len(staged.ops) == 0 {
		return nil
	}
	return K.db.Update(func(tx *bolt.Tx) error {
		for _, op := range staged.ops {
			if op.unset {
				bucket := tx.Bucket([]byte(op.table))
				if bucket == nil {
					continue
				}
				if err := bucket.Delete([]byte(op.key)); err != nil {
					return err
				}
				continue
			}
			bucket, err := tx.CreateBucketIfNotExists([]byte(op.table))
			if err != nil {
				return err
			}
			if err := bucket.Put([]byte(op.key), op.value); err != nil {
				return err
			}
		}
		return nil
	})
}

// Close closes the database connection.
func (K *boltDB) Close() (err error) {
	return K.db.Close()
}

// CryptSet encrypts and sets a value in the specified table.
func (K *boltDB) CryptSet(table, key string, value interface{}) (err error) {
	return K.set(table, key, value, true)
}

// Sets the value for the given key in the specified table.
// It creates the table if it doesn't exist.
func (K *boltDB) Set(table, key string, value interface{}) (err error) {
	return K.set(table, key, value, false)
}

// Sets the value for the given key in the specified table, optionally encrypting it.
// The value is encoded before being stored.
func (K *boltDB) set(table, key string, value interface{}, encrypt_value bool) (err error) {
	v, err := K.encoder.pack(value, encrypt_value)
	if err != nil {
		return err
	}
	return K.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(table))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), v)
	})
}

// CryptReset resets the database by deleting all encrypted keys and the KVLite bucket.
func CryptReset(filename string) (err error) {
	db, err := open(filename)
	if err != nil {
		return err
	}

	db.Set("KVLite", "Reset", true)

	// Find the encrypted keys in one read, then delete them and the KVLite
	// bucket in one write.
	crypted_keys := make(map[string][][]byte)
	err = db.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			if string(name) == "KVLite" {
				return nil
			}
			return b.ForEach(func(k, v []byte) error {
				if len(v) > 0 && v[0] == 1 {
					crypted_keys[string(name)] = append(crypted_keys[string(name)], append([]byte(nil), k...))
				}
				return nil
			})
		})
	})
	if err != nil {
		db.Close()
		return err
	}

	err = db.db.Update(func(tx *bolt.Tx) error {
		for t, keys := range crypted_keys {
			bucket := tx.Bucket([]byte(t))
			if bucket == nil {
				continue
			}
			for _, k := range keys {
				if err := bucket.Delete(k); err != nil {
					return err
				}
			}
		}
		return tx.DeleteBucket([]byte("KVLite"))
	})
	if err != nil {
		db.Close()
		return err
	}
	return db.Close()
}

// open opens a boltDB database. It returns the database instance and any error.
// It handles bolt.ErrTimeout by returning ErrLocked.
func open(filename string) (DB *boltDB, err error) {
	db, err := bolt.Open(filename, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		if err == bolt.ErrTimeout {
			err = ErrLocked
		}
		return nil, err
	}
	return &boltDB{db: db}, nil
}

// Open opens or creates a database file and performs necessary setup.
// It handles reset, decryption, and sets up the encoder.
func Open(filename string, padlock ...byte) (Store, error) {
	db, err := open(filename)
	if err != nil {
		return nil, err
	}

	found, err := db.Get("KVLite", "Reset", nil)
	if err != nil {
		return nil, err
	}

	if found {
		db.Close()
		err = CryptReset(filename)
		if err != nil {
			return nil, err
		}
		db, err = open(filename)
		if err != nil {
			return nil, err
		}
	}

	var X *xLock
	_, err = db.Get("KVLite", "X", &X)
	if err != nil {
		return nil, err
	}
	if X == nil {
		X = new(xLock)
	}

	db.encoder, err = X.dbunlocker(padlock)
	if err != nil {
		db.Close()
		return nil, err
	}
	err = db.Set("KVLite", "X", &X)
	return db, err
}

// ErrNoPadlock is ChangePadlock on a database that has no padlock yet.
var ErrNoPadlock = errors.New("database has no padlock to change: open it once first")

// ChangePadlock moves the database file at filename from oldPadlock to
// newPadlock, without touching the values it holds.
//
// The values are encrypted under a random data key, and the padlock only
// wraps that key, so changing the padlock re-wraps one key: nothing else is
// re-encrypted. The new wrapping is checked to give back the same key before
// it is saved, and it is saved in one write, so a failure leaves the
// database on oldPadlock. ErrBadPadlock when oldPadlock does not open it.
// The file must not be open elsewhere.
func ChangePadlock(filename string, oldPadlock, newPadlock []byte) error {
	db, err := open(filename)
	if err != nil {
		return err
	}
	defer db.Close()

	var X *xLock
	if _, err = db.Get("KVLite", "X", &X); err != nil {
		return err
	}
	if X == nil || len(X.Msg) == 0 {
		return ErrNoPadlock
	}
	key, err := X.dbunlocker(oldPadlock)
	if err != nil {
		return err
	}
	// Copied: dblocker appends to the slice it is given.
	key = append([]byte(nil), key[0:32]...)

	next := new(xLock)
	next.dblocker(append([]byte(nil), key...), newPadlock)
	check, err := next.dbunlocker(newPadlock)
	if err != nil || !bytes.Equal(check[0:32], key) {
		return fmt.Errorf("the new padlock did not give back the database key, so nothing was changed")
	}
	return db.Set("KVLite", "X", &next)
}

// OpenRekeying opens filename with target, the padlock it should be under.
// When target does not open it, each of fallbacks is tried in turn (the
// padlocks it may still be under from before); the first that opens it is
// moved to target with ChangePadlock, after backup runs, if given, so the
// caller can copy the file aside first. note says what happened when there
// is something to say: moved to target, or could not be and why, in which
// case the store is open under that fallback and the next open tries again.
// ErrBadPadlock when none of them opens it.
func OpenRekeying(filename string, target []byte, fallbacks [][]byte, backup func() error) (db Store, note string, err error) {
	db, err = Open(filename, target...)
	if err != ErrBadPadlock {
		return db, "", err
	}
	for i, fb := range fallbacks {
		if len(fb) == 0 {
			continue
		}
		db, err = Open(filename, fb...)
		if err == ErrBadPadlock {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		db.Close()
		if backup != nil {
			if berr := backup(); berr != nil {
				db, err = Open(filename, fb...)
				return db, fmt.Sprintf("not moved to the new padlock: could not copy it aside first (%v)", berr), err
			}
		}
		if cerr := ChangePadlock(filename, fb, target); cerr != nil {
			db, err = Open(filename, fb...)
			return db, fmt.Sprintf("not moved to the new padlock (%v)", cerr), err
		}
		db, err = Open(filename, target...)
		if err != nil {
			return nil, "", fmt.Errorf("moved to the new padlock but it did not open after: %w", err)
		}
		return db, fmt.Sprintf("moved to the new padlock from padlock %d", i+1), nil
	}
	return nil, "", ErrBadPadlock
}
